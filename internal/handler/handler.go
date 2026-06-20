package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"task-team-api/internal/middleware"
	"task-team-api/internal/models"
	"task-team-api/internal/repository"
	"task-team-api/internal/service"
)

type Handler struct {
	svc *service.Service
}

func New(svc *service.Service) *Handler { return &Handler{svc: svc} }

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type createTeamRequest struct {
	Name string `json:"name"`
}

type inviteRequest struct {
	Email string      `json:"email"`
	Role  models.Role `json:"role"`
}

type createTaskRequest struct {
	TeamID      int64             `json:"team_id"`
	Title       string            `json:"title"`
	Description *string           `json:"description"`
	Status      models.TaskStatus `json:"status"`
	AssigneeID  *int64            `json:"assignee_id"`
}

type updateTaskRequest struct {
	Title       *string            `json:"title"`
	Description *string            `json:"description"`
	Status      *models.TaskStatus `json:"status"`
	AssigneeID  *int64             `json:"assignee_id"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	user, err := h.svc.Register(r.Context(), req.Name, req.Email, req.Password)
	if err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, user)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	token, user, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"token": token, "user": user})
}

func (h *Handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req createTeamRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	team, err := h.svc.CreateTeam(r.Context(), userID, req.Name)
	if err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, team)
}

func (h *Handler) ListTeams(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	teams, err := h.svc.ListTeams(r.Context(), userID)
	if err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, teams)
}

func (h *Handler) InviteUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	teamID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || teamID <= 0 {
		http.Error(w, "invalid team id", http.StatusBadRequest)
		return
	}
	var req inviteRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.InviteUser(r.Context(), userID, teamID, req.Email, req.Role); err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "invited"})
}

func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req createTaskRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	task, err := h.svc.CreateTask(r.Context(), userID, models.Task{TeamID: req.TeamID, Title: req.Title, Description: req.Description, Status: req.Status, AssigneeID: req.AssigneeID})
	if err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, task)
}

func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	teamID, err := strconv.ParseInt(r.URL.Query().Get("team_id"), 10, 64)
	if err != nil || teamID <= 0 {
		http.Error(w, "team_id is required", http.StatusBadRequest)
		return
	}
	var status *models.TaskStatus
	if raw := r.URL.Query().Get("status"); raw != "" {
		parsed := models.TaskStatus(raw)
		status = &parsed
	}
	var assigneeID *int64
	if raw := r.URL.Query().Get("assignee_id"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			http.Error(w, "invalid assignee_id", http.StatusBadRequest)
			return
		}
		assigneeID = &parsed
	}
	page := queryInt(r, "page", 1)
	limit := queryInt(r, "limit", 20)
	tasks, err := h.svc.ListTasks(r.Context(), userID, teamID, status, assigneeID, page, limit)
	if err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"page": page, "limit": limit, "items": tasks})
}

func (h *Handler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	taskID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || taskID <= 0 {
		http.Error(w, "invalid task id", http.StatusBadRequest)
		return
	}
	var req updateTaskRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	task, err := h.svc.UpdateTask(r.Context(), userID, taskID, req.Title, req.Description, req.Status, req.AssigneeID)
	if err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, task)
}

func (h *Handler) TaskHistory(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	taskID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || taskID <= 0 {
		http.Error(w, "invalid task id", http.StatusBadRequest)
		return
	}
	history, err := h.svc.TaskHistory(r.Context(), userID, taskID)
	if err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, history)
}

func (h *Handler) TeamSummaries(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.TeamSummaries(r.Context())
	if err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, items)
}

func (h *Handler) TopCreators(w http.ResponseWriter, r *http.Request) {
	month := time.Now()
	if raw := r.URL.Query().Get("month"); raw != "" {
		parsed, err := time.Parse("2006-01", raw)
		if err != nil {
			http.Error(w, "month must be YYYY-MM", http.StatusBadRequest)
			return
		}
		month = parsed
	}
	items, err := h.svc.TopCreatorsByTeam(r.Context(), month)
	if err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, items)
}

func (h *Handler) InvalidAssignees(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.InvalidAssigneeTasks(r.Context())
	if err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, items)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

func respondJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, service.ErrBadRequest):
		status = http.StatusBadRequest
	case errors.Is(err, service.ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, service.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, repository.ErrNotFound):
		status = http.StatusNotFound
	}
	respondJSON(w, status, map[string]string{"error": err.Error()})
}

func queryInt(r *http.Request, key string, fallback int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return parsed
}
