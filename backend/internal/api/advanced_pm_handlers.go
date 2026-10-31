package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/takayoshi-u/polaris/backend/internal/domain"
	"github.com/takayoshi-u/polaris/backend/internal/store"
)

func (s *Server) getUserID(r *http.Request) string {
	u := currentUser(r)
	if u == nil {
		return ""
	}
	return u.ID
}

func (s *Server) respondJSON(w http.ResponseWriter, status int, payload any) {
	writeJSON(w, status, payload)
}

func (s *Server) respondError(w http.ResponseWriter, err error) {
	writeError(w, err)
}


// -------------------------------------------------------------
// 1. Task Dependencies Handlers
// -------------------------------------------------------------

func (s *Server) handleListTaskDependencies(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	deps, err := s.store.ListTaskDependencies(r.Context(), projectID)
	if err != nil {
		s.respondError(w, err)
		return
	}
	if deps == nil {
		deps = []domain.TaskDependency{}
	}
	s.respondJSON(w, http.StatusOK, deps)
}

func (s *Server) handleCreateTaskDependency(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	userID := s.getUserID(r)

	var req struct {
		PredecessorID  string                `json:"predecessor_id"`
		SuccessorID    string                `json:"successor_id"`
		DependencyType domain.DependencyType `json:"dependency_type"`
		LagDays        int                   `json:"lag_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, fmt.Errorf("%w: invalid request body", store.ErrBadRequest))
		return
	}

	dep, err := s.store.CreateTaskDependency(r.Context(), projectID, userID, req.PredecessorID, req.SuccessorID, req.DependencyType, req.LagDays)
	if err != nil {
		s.respondError(w, err)
		return
	}
	s.respondJSON(w, http.StatusCreated, dep)
}

func (s *Server) handleDeleteTaskDependency(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	depID := r.PathValue("depId")
	userID := s.getUserID(r)

	if err := s.store.DeleteTaskDependency(r.Context(), projectID, depID, userID); err != nil {
		s.respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// -------------------------------------------------------------
// 2. Sprints Handlers
// -------------------------------------------------------------

func (s *Server) handleListSprints(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	sprints, err := s.store.ListSprints(r.Context(), projectID)
	if err != nil {
		s.respondError(w, err)
		return
	}
	if sprints == nil {
		sprints = []domain.Sprint{}
	}
	s.respondJSON(w, http.StatusOK, sprints)
}

func (s *Server) handleCreateSprint(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	userID := s.getUserID(r)

	var in store.SprintCreateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		s.respondError(w, fmt.Errorf("%w: invalid request body", store.ErrBadRequest))
		return
	}

	sp, err := s.store.CreateSprint(r.Context(), projectID, userID, in)
	if err != nil {
		s.respondError(w, err)
		return
	}
	s.respondJSON(w, http.StatusCreated, sp)
}

func (s *Server) handleGetSprint(w http.ResponseWriter, r *http.Request) {
	sprintID := r.PathValue("sprintId")
	userID := s.getUserID(r)

	sp, err := s.store.GetSprint(r.Context(), sprintID, userID)
	if err != nil {
		s.respondError(w, err)
		return
	}
	s.respondJSON(w, http.StatusOK, sp)
}

func (s *Server) handleUpdateSprint(w http.ResponseWriter, r *http.Request) {
	sprintID := r.PathValue("sprintId")
	userID := s.getUserID(r)

	var in store.SprintUpdateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		s.respondError(w, fmt.Errorf("%w: invalid request body", store.ErrBadRequest))
		return
	}

	sp, err := s.store.UpdateSprint(r.Context(), sprintID, userID, in)
	if err != nil {
		s.respondError(w, err)
		return
	}
	s.respondJSON(w, http.StatusOK, sp)
}

func (s *Server) handleDeleteSprint(w http.ResponseWriter, r *http.Request) {
	sprintID := r.PathValue("sprintId")
	userID := s.getUserID(r)

	if err := s.store.DeleteSprint(r.Context(), sprintID, userID); err != nil {
		s.respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetSprintBurndown(w http.ResponseWriter, r *http.Request) {
	sprintID := r.PathValue("sprintId")
	userID := s.getUserID(r)

	burndown, err := s.store.GetSprintBurndown(r.Context(), sprintID, userID)
	if err != nil {
		s.respondError(w, err)
		return
	}
	s.respondJSON(w, http.StatusOK, burndown)
}

// -------------------------------------------------------------
// 3. Activity Logs Handlers
// -------------------------------------------------------------

func (s *Server) handleListActivityLogs(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	q := r.URL.Query()

	var entityType, entityID *string
	if val := q.Get("entity_type"); val != "" {
		entityType = &val
	}
	if val := q.Get("entity_id"); val != "" {
		entityID = &val
	}
	limit := 50
	if lStr := q.Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}

	logs, err := s.store.ListActivities(r.Context(), projectID, entityType, entityID, limit)
	if err != nil {
		s.respondError(w, err)
		return
	}
	if logs == nil {
		logs = []domain.ActivityLog{}
	}
	s.respondJSON(w, http.StatusOK, logs)
}

// -------------------------------------------------------------
// 4. Project Wiki Handlers
// -------------------------------------------------------------

func (s *Server) handleListWikiPages(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	pages, err := s.store.ListWikiPages(r.Context(), projectID)
	if err != nil {
		s.respondError(w, err)
		return
	}
	if pages == nil {
		pages = []domain.WikiPage{}
	}
	s.respondJSON(w, http.StatusOK, pages)
}

func (s *Server) handleCreateWikiPage(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	userID := s.getUserID(r)

	var in store.WikiPageCreateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		s.respondError(w, fmt.Errorf("%w: invalid request body", store.ErrBadRequest))
		return
	}

	page, err := s.store.CreateWikiPage(r.Context(), projectID, userID, in)
	if err != nil {
		s.respondError(w, err)
		return
	}
	s.respondJSON(w, http.StatusCreated, page)
}

func (s *Server) handleGetWikiPage(w http.ResponseWriter, r *http.Request) {
	wikiID := r.PathValue("wikiId")
	userID := s.getUserID(r)

	page, err := s.store.GetWikiPage(r.Context(), wikiID, userID)
	if err != nil {
		s.respondError(w, err)
		return
	}
	s.respondJSON(w, http.StatusOK, page)
}

func (s *Server) handleUpdateWikiPage(w http.ResponseWriter, r *http.Request) {
	wikiID := r.PathValue("wikiId")
	userID := s.getUserID(r)

	var in store.WikiPageUpdateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		s.respondError(w, fmt.Errorf("%w: invalid request body", store.ErrBadRequest))
		return
	}

	page, err := s.store.UpdateWikiPage(r.Context(), wikiID, userID, in)
	if err != nil {
		s.respondError(w, err)
		return
	}
	s.respondJSON(w, http.StatusOK, page)
}

func (s *Server) handleDeleteWikiPage(w http.ResponseWriter, r *http.Request) {
	wikiID := r.PathValue("wikiId")
	userID := s.getUserID(r)

	if err := s.store.DeleteWikiPage(r.Context(), wikiID, userID); err != nil {
		s.respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// -------------------------------------------------------------
// 5. Inbound Webhooks Handlers
// -------------------------------------------------------------

func (s *Server) handleListInboundWebhooks(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	hooks, err := s.store.ListInboundWebhooks(r.Context(), projectID)
	if err != nil {
		s.respondError(w, err)
		return
	}
	if hooks == nil {
		hooks = []domain.InboundWebhook{}
	}
	s.respondJSON(w, http.StatusOK, hooks)
}

func (s *Server) handleCreateInboundWebhook(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	userID := s.getUserID(r)

	var req struct {
		Name            string  `json:"name"`
		DefaultIssueID  *string `json:"default_issue_id"`
		DefaultStatusID int     `json:"default_status_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, fmt.Errorf("%w: invalid request body", store.ErrBadRequest))
		return
	}

	hook, err := s.store.CreateInboundWebhook(r.Context(), projectID, userID, req.Name, req.DefaultIssueID, req.DefaultStatusID)
	if err != nil {
		s.respondError(w, err)
		return
	}
	s.respondJSON(w, http.StatusCreated, hook)
}

func (s *Server) handleDeleteInboundWebhook(w http.ResponseWriter, r *http.Request) {
	webhookID := r.PathValue("webhookId")
	userID := s.getUserID(r)

	if err := s.store.DeleteInboundWebhook(r.Context(), webhookID, userID); err != nil {
		s.respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Public Inbound Webhook Receiver
func (s *Server) handleProcessInboundWebhook(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		s.respondError(w, fmt.Errorf("%w: token is required", store.ErrUnauthorized))
		return
	}

	var payload domain.InboundWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		s.respondError(w, fmt.Errorf("%w: invalid json payload", store.ErrBadRequest))
		return
	}

	task, err := s.store.ProcessInboundWebhook(r.Context(), token, payload)
	if err != nil {
		s.respondError(w, err)
		return
	}
	s.respondJSON(w, http.StatusCreated, task)
}

// -------------------------------------------------------------
// 6. CSV Export & Import Handlers
// -------------------------------------------------------------

func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	userID := s.getUserID(r)

	csvData, err := s.store.ExportProjectCSV(r.Context(), projectID, userID)
	if err != nil {
		s.respondError(w, err)
		return
	}

	filename := fmt.Sprintf("project_%s_export.csv", projectID[:8])
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(csvData)
}

func (s *Server) handleImportCSV(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	userID := s.getUserID(r)

	var reader = r.Body
	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "multipart/form-data") {
		err := r.ParseMultipartForm(10 << 20) // 10MB
		if err != nil {
			s.respondError(w, fmt.Errorf("%w: invalid multipart form", store.ErrBadRequest))
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			s.respondError(w, fmt.Errorf("%w: file field is required in multipart form", store.ErrBadRequest))
			return
		}
		defer file.Close()
		reader = file
	}

	result, err := s.store.ImportProjectCSV(r.Context(), projectID, userID, reader)
	if err != nil {
		s.respondError(w, err)
		return
	}
	s.respondJSON(w, http.StatusOK, result)
}
