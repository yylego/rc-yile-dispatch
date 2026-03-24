package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/yylego/rc-yile-dispatch/internal/model"
	"github.com/yylego/rc-yile-dispatch/internal/store"
)

type Handler struct {
	store *store.Store
}

func New(s *store.Store) *Handler {
	return &Handler{store: s}
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

// Submit handles POST /api/dispatch — accepts a notification task
func (h *Handler) Submit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrJSON(w, http.StatusMethodNotAllowed, "method not accepted")
		return
	}

	var req SubmitReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrJSON(w, http.StatusBadRequest, "bad request body")
		return
	}

	if req.TargetURL == "" {
		writeErrJSON(w, http.StatusBadRequest, "targetUrl is needed")
		return
	}
	if req.Method == "" {
		req.Method = http.MethodPost
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

	if err := h.store.CreateTask(r.Context(), task); err != nil {
		writeErrJSON(w, http.StatusInternalServerError, "save task failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":      task.ID,
		"status":  task.Status,
		"message": "task accepted",
	})
}

// GetTask handles GET /api/task/{id} — check task status
func (h *Handler) GetTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrJSON(w, http.StatusMethodNotAllowed, "method not accepted")
		return
	}

	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		writeErrJSON(w, http.StatusBadRequest, "bad id")
		return
	}

	task, err := h.store.GetTask(r.Context(), uint(id))
	if err != nil {
		writeErrJSON(w, http.StatusNotFound, "task not found")
		return
	}

	writeJSON(w, http.StatusOK, task)
}

// ListTasks handles GET /api/tasks — list tasks with pagination
func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrJSON(w, http.StatusMethodNotAllowed, "method not accepted")
		return
	}

	status := r.URL.Query().Get("status")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	tasks, total, err := h.store.ListTasks(r.Context(), status, page, pageSize)
	if err != nil {
		writeErrJSON(w, http.StatusInternalServerError, "list failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": tasks,
		"total": total,
		"page":  page,
	})
}

type errResponse struct {
	Message string `json:"message"`
}

func writeErrJSON(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(&errResponse{Message: message})
}

func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}
