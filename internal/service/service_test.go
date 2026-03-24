package service_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/yylego/must"
	"github.com/yylego/rc-yile-dispatch/internal/model"
	"github.com/yylego/rc-yile-dispatch/internal/service"
	"github.com/yylego/rese"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	restyv2 "github.com/go-resty/resty/v2"
)

var testBaseURL string
var testQuit chan struct{}

func TestMain(m *testing.M) {
	dsn := fmt.Sprintf("file:db-%s?mode=memory&cache=shared", uuid.New().String())
	db := rese.P1(gorm.Open(sqlite.Open(dsn), &gorm.Config{}))
	must.Done(db.AutoMigrate(&model.Task{}))

	testQuit = make(chan struct{})

	testAddress := ":18088"
	go service.Run(db, testAddress, testQuit)
	time.Sleep(time.Second)

	testBaseURL = "http://localhost" + testAddress

	code := m.Run()

	close(testQuit)
	time.Sleep(time.Millisecond * 500)
	os.Exit(code)
}

func TestHealth(t *testing.T) {
	resp := rese.C1(restyv2.New().R().Get(testBaseURL + "/health"))
	require.Equal(t, 200, resp.StatusCode())
	require.Contains(t, resp.String(), "ok")
}

func TestSubmitAndDispatchSuccess(t *testing.T) {
	var received atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	var submitResult map[string]any
	resp := rese.C1(restyv2.New().R().
		SetResult(&submitResult).
		SetBody(map[string]any{
			"method":    "POST",
			"targetUrl": target.URL,
			"headers":   map[string]string{"X-Test": "dispatch"},
			"body":      `{"msg":"test"}`,
		}).
		Post(testBaseURL + "/api/dispatch"))
	require.Equal(t, 200, resp.StatusCode())
	require.Equal(t, "pending", submitResult["status"])
	t.Log("task id:", submitResult["id"])

	time.Sleep(time.Second * 3)

	require.True(t, received.Load() >= 1)

	var taskResult map[string]any
	resp = rese.C1(restyv2.New().R().
		SetResult(&taskResult).
		SetQueryParam("id", "1").
		Get(testBaseURL + "/api/task"))
	require.Equal(t, 200, resp.StatusCode())
	require.Equal(t, "success", taskResult["Status"])
}

func TestSubmitBadRequest(t *testing.T) {
	resp := rese.C1(restyv2.New().R().
		SetBody(map[string]any{"method": "POST"}).
		Post(testBaseURL + "/api/dispatch"))
	require.Equal(t, 400, resp.StatusCode())
	require.Contains(t, resp.String(), "targetUrl is needed")
}

func TestDispatchRetryAndDeadline(t *testing.T) {
	var attempts atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer target.Close()

	var submitResult map[string]any
	resp := rese.C1(restyv2.New().R().
		SetResult(&submitResult).
		SetBody(map[string]any{
			"method":     "POST",
			"targetUrl":  target.URL,
			"maxRetries": 1,
		}).
		Post(testBaseURL + "/api/dispatch"))
	require.Equal(t, 200, resp.StatusCode())
	taskID := "2"

	time.Sleep(time.Second * 5)

	var taskResult map[string]any
	resp = rese.C1(restyv2.New().R().
		SetResult(&taskResult).
		SetQueryParam("id", taskID).
		Get(testBaseURL + "/api/task"))
	require.Equal(t, 200, resp.StatusCode())
	require.Equal(t, "deadline", taskResult["Status"])
	require.True(t, attempts.Load() >= 1)
	t.Log("attempts:", attempts.Load())
}

func TestListTasks(t *testing.T) {
	var result map[string]any
	resp := rese.C1(restyv2.New().R().
		SetResult(&result).
		SetQueryParams(map[string]string{"page": "1", "pageSize": "10"}).
		Get(testBaseURL + "/api/tasks"))
	require.Equal(t, 200, resp.StatusCode())
	t.Log("total:", result["total"])
}
