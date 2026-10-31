package api

import (
	"encoding/json"
	"net/http"
)

type TypeRequest struct {
	Name         string `json:"name"`
	Color        string `json:"color"`
	DisplayOrder *int   `json:"display_order,omitempty"`
}

// System Common Types Handlers

func (s *Server) handleListSystemTypes(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListTypes(r.Context(), nil)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateSystemType(w http.ResponseWriter, r *http.Request) {
	var body TypeRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	displayOrder := 0
	if body.DisplayOrder != nil {
		displayOrder = *body.DisplayOrder
	}
	it, err := s.store.CreateType(r.Context(), nil, body.Name, body.Color, displayOrder)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, it)
}

func (s *Server) handleUpdateSystemType(w http.ResponseWriter, r *http.Request) {
	typeID := r.PathValue("typeId")
	existing, err := s.store.GetType(r.Context(), typeID)
	if err != nil {
		writeError(w, err)
		return
	}
	if existing.ProjectID != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "cannot update project-specific type via system endpoint"})
		return
	}

	var body TypeRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}

	it, err := s.store.UpdateType(r.Context(), typeID, body.Name, body.Color, body.DisplayOrder)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, it)
}

func (s *Server) handleDeleteSystemType(w http.ResponseWriter, r *http.Request) {
	typeID := r.PathValue("typeId")
	existing, err := s.store.GetType(r.Context(), typeID)
	if err != nil {
		writeError(w, err)
		return
	}
	if existing.ProjectID != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "cannot delete project-specific type via system endpoint"})
		return
	}

	if err := s.store.DeleteType(r.Context(), typeID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Project-scoped Types Handlers

func (s *Server) handleListProjectTypes(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	list, err := s.store.ListTypes(r.Context(), &projectID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateProjectType(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	var body TypeRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	displayOrder := 0
	if body.DisplayOrder != nil {
		displayOrder = *body.DisplayOrder
	}
	it, err := s.store.CreateType(r.Context(), &projectID, body.Name, body.Color, displayOrder)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, it)
}

func (s *Server) handleUpdateProjectType(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	typeID := r.PathValue("typeId")

	existing, err := s.store.GetType(r.Context(), typeID)
	if err != nil {
		writeError(w, err)
		return
	}
	if existing.ProjectID == nil || *existing.ProjectID != projectID {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "cannot modify system or other project type from this project"})
		return
	}

	var body TypeRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}

	it, err := s.store.UpdateType(r.Context(), typeID, body.Name, body.Color, body.DisplayOrder)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, it)
}

func (s *Server) handleDeleteProjectType(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	typeID := r.PathValue("typeId")

	existing, err := s.store.GetType(r.Context(), typeID)
	if err != nil {
		writeError(w, err)
		return
	}
	if existing.ProjectID == nil || *existing.ProjectID != projectID {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "cannot delete system or other project type from this project"})
		return
	}

	if err := s.store.DeleteType(r.Context(), typeID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
