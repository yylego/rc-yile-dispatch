package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/yylego/rc-yile-dispatch/internal/model"
	"github.com/yylego/rc-yile-dispatch/internal/store"
	"github.com/yylego/reggin/warpginhandle"
)

type Handler struct {
	store *store.Store
}

func New(s *store.Store) *Handler {
	return &Handler{store: s}
}

type faultResp struct {
	Message string `json:"message"`
}

func newFaultResp(ctx *gin.Context, cause error) *faultResp {
	return &faultResp{Message: cause.Error()}
}

// SubmitReq is the request body to submit a notification task
type SubmitReq struct {
	Method     string            `json:"method"`               // HTTP method
	TargetURL  string            `json:"targetUrl"`            // destination URL
	Headers    map[string]string `json:"headers,omitempty"`    // optional headers
	Body       string            `json:"body,omitempty"`       // optional body
	MaxRetries int               `json:"maxRetries,omitempty"` // optional, defaults to 5
	Callback   string            `json:"callback,omitempty"`   // optional business tag
}

type SubmitResp struct {
	ID      uint             `json:"id"`
	Status  model.TaskStatus `json:"status"`
	Message string           `json:"message"`
}

// Submit returns the gin handler for POST /api/dispatch
func (h *Handler) Submit() gin.HandlerFunc {
	return warpginhandle.HX[SubmitReq, *SubmitResp](func(ctx *gin.Context, req *SubmitReq) (*SubmitResp, int, error) {
		if req.TargetURL == "" {
			return nil, http.StatusBadRequest, errors.New("targetUrl is needed")
		}
		if req.Method == "" {
			req.Method = "POST"
		}
		if req.MaxRetries <= 0 {
			req.MaxRetries = 5
		}

		headersJSON := ""
		if len(req.Headers) > 0 {
			b, _ := json.Marshal(req.Headers)
			headersJSON = string(b)
		}

		task := &model.Task{
			Method:     req.Method,
			TargetURL:  req.TargetURL,
			Headers:    headersJSON,
			Body:       req.Body,
			Status:     model.StatusPending,
			MaxRetries: req.MaxRetries,
			NextRunAt:  time.Now().Unix(),
			Callback:   req.Callback,
		}

		if err := h.store.CreateTask(ctx.Request.Context(), task); err != nil {
			return nil, http.StatusInternalServerError, errors.New("save task failed")
		}

		return &SubmitResp{
			ID:      task.ID,
			Status:  task.Status,
			Message: "task accepted",
		}, http.StatusOK, nil
	}, newFaultResp)
}

type GetTaskReq struct {
	ID string `json:"id"`
}

// GetTask returns the gin handler for GET /api/task
func (h *Handler) GetTask() gin.HandlerFunc {
	return warpginhandle.H1[GetTaskReq, *model.Task](func(ctx *gin.Context, req *GetTaskReq) (*model.Task, int, error) {
		id, err := strconv.ParseUint(req.ID, 10, 64)
		if err != nil || id == 0 {
			return nil, http.StatusBadRequest, errors.New("bad id")
		}

		task, err := h.store.GetTask(ctx.Request.Context(), uint(id))
		if err != nil {
			return nil, http.StatusNotFound, errors.New("task not found")
		}
		return task, http.StatusOK, nil
	}, warpginhandle.QueryJson[GetTaskReq], newFaultResp)
}

type ListTasksReq struct {
	Status   string `json:"status"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

type ListTasksResp struct {
	Items []*model.Task `json:"items"`
	Total int64         `json:"total"`
	Page  int           `json:"page"`
}

// ListTasks returns the gin handler for GET /api/tasks
func (h *Handler) ListTasks() gin.HandlerFunc {
	return warpginhandle.H1[ListTasksReq, *ListTasksResp](func(ctx *gin.Context, req *ListTasksReq) (*ListTasksResp, int, error) {
		if req.Page <= 0 {
			req.Page = 1
		}
		if req.PageSize <= 0 {
			req.PageSize = 20
		}

		tasks, total, err := h.store.ListTasks(ctx.Request.Context(), req.Status, req.Page, req.PageSize)
		if err != nil {
			return nil, http.StatusInternalServerError, errors.New("list failed")
		}

		return &ListTasksResp{
			Items: tasks,
			Total: total,
			Page:  req.Page,
		}, http.StatusOK, nil
	}, warpginhandle.QueryJson[ListTasksReq], newFaultResp)
}

type HealthResp struct {
	Status string `json:"status"`
}

// Health returns the gin handler for GET /health
func (h *Handler) Health() gin.HandlerFunc {
	return warpginhandle.H0[*HealthResp](func(ctx *gin.Context) (*HealthResp, int, error) {
		return &HealthResp{Status: "ok"}, http.StatusOK, nil
	}, newFaultResp)
}
