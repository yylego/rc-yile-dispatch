package service

import (
	"log"
	"net/http"

	"github.com/yylego/must"
	"github.com/yylego/rc-yile-dispatch/internal/dispatch"
	"github.com/yylego/rc-yile-dispatch/internal/handlers"
	"github.com/yylego/rc-yile-dispatch/internal/store"
	"gorm.io/gorm"
)

func Run(db *gorm.DB, address string, quit <-chan struct{}) {
	s := store.New(db)
	h := handlers.New(s)
	d := dispatch.NewDispatcher(s)

	d.Start()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/dispatch", h.Submit)
	mux.HandleFunc("/api/task", h.GetTask)
	mux.HandleFunc("/api/tasks", h.ListTasks)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	log.Printf("[service] listening on %s", address)

	go func() {
		must.Done(http.ListenAndServe(address, mux))
	}()

	<-quit

	log.Println("[service] shutting down...")
	d.Stop()
	log.Println("[service] done")
}
