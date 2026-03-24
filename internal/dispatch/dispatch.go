package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yylego/rc-yile-dispatch/internal/store"
)

type Dispatcher struct {
	store      *store.Store
	client     *http.Client
	pollPeriod time.Duration
	batchSize  int
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

func NewDispatcher(s *store.Store) *Dispatcher {
	return &Dispatcher{
		store: s,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		pollPeriod: time.Second,
		batchSize:  10,
		stopCh:     make(chan struct{}),
	}
}

// Start begins the background dispatch loop
func (d *Dispatcher) Start() {
	d.wg.Add(1)
	go d.run()
	log.Println("[dispatcher] background dispatch started")
}

// Stop signals the dispatch loop to terminate and waits
func (d *Dispatcher) Stop() {
	close(d.stopCh)
	d.wg.Wait()
	log.Println("[dispatcher] stopped")
}

func (d *Dispatcher) run() {
	defer d.wg.Done()
	tick := time.NewTicker(d.pollPeriod)
	defer tick.Stop()

	for {
		select {
		case <-d.stopCh:
			return
		case <-tick.C:
			d.poll()
		}
	}
}

func (d *Dispatcher) poll() {
	ctx := context.Background()

	tasks, err := d.store.FetchPendingTasks(ctx, d.batchSize)
	if err != nil {
		log.Printf("[dispatcher] fetch tasks: %v", err)
		return
	}

	for _, task := range tasks {
		if err := d.store.MarkRunning(ctx, task.ID); err != nil {
			log.Printf("[dispatcher] mark running task %d: %v", task.ID, err)
			continue
		}

		err := d.dispatch(task.Method, task.TargetURL, task.Headers, task.Body)
		if err != nil {
			log.Printf("[dispatcher] task %d dispatch failed (attempt %d): %v", task.ID, task.Retries+1, err)
			if markErr := d.store.MarkFailed(ctx, task.ID, task.Retries+1, task.MaxRetries, err.Error()); markErr != nil {
				log.Printf("[dispatcher] mark failed task %d: %v", task.ID, markErr)
			}
		} else {
			log.Printf("[dispatcher] task %d dispatched OK", task.ID)
			if markErr := d.store.MarkSuccess(ctx, task.ID); markErr != nil {
				log.Printf("[dispatcher] mark success task %d: %v", task.ID, markErr)
			}
		}
	}
}

func (d *Dispatcher) dispatch(method, targetURL, headersJSON, body string) error {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, targetURL, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	// set custom headers
	if headersJSON != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(headersJSON), &headers); err == nil {
			for k, v := range headers {
				req.Header.Set(k, v)
			}
		}
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("non-2xx response: %d", resp.StatusCode)
}
