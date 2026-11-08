package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/takayoshi-u/polaris/backend/internal/domain"
)

func TestProjectArchive_ReadOnlyAndRestore(t *testing.T) {
	rig := setupTestRig(t)
	defer rig.db.Close()

	rec := sprintDurDo(t, rig, http.MethodPost, "/api/v1/projects", map[string]any{"name": "arch"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	p := decodeJSON[domain.Project](t, rec)
	base := "/api/v1/projects/" + p.ID

	rec = sprintDurDo(t, rig, http.MethodPost, base+"/sprints", map[string]any{"name": "S1"})
	sp := decodeJSON[domain.Sprint](t, rec)

	rec = sprintDurDo(t, rig, http.MethodPut, base+"/archive", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("archive: %d %s", rec.Code, rec.Body.String())
	}
	ap := decodeJSON[domain.Project](t, rec)
	if !ap.IsArchived || ap.ArchivedAt == nil || ap.ArchivedBy == nil {
		t.Fatalf("expected archived fields, got %+v", ap)
	}
	if rec = sprintDurDo(t, rig, http.MethodPut, base+"/archive", nil); rec.Code != http.StatusConflict {
		t.Fatalf("double archive expected 409, got %d", rec.Code)
	}

	// Mutations blocked, reads allowed.
	for _, c := range []struct{ method, path string }{
		{http.MethodPost, base + "/issues"},
		{http.MethodPost, base + "/sprints"},
		{http.MethodPost, base + "/wiki"},
		{http.MethodPut, base},
		{http.MethodPut, "/api/v1/sprints/" + sp.ID},
		{http.MethodDelete, "/api/v1/sprints/" + sp.ID},
	} {
		rec = sprintDurDo(t, rig, c.method, c.path, map[string]any{"title": "x", "name": "x"})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s %s: expected 403, got %d", c.method, c.path, rec.Code)
		}
	}
	if rec = sprintDurDo(t, rig, http.MethodGet, base+"/sprints", nil); rec.Code != http.StatusOK {
		t.Fatalf("read expected 200, got %d", rec.Code)
	}

	rec = sprintDurDo(t, rig, http.MethodPut, base+"/unarchive", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("unarchive: %d %s", rec.Code, rec.Body.String())
	}
	up := decodeJSON[domain.Project](t, rec)
	if up.IsArchived || up.ArchivedAt != nil || up.ArchivedBy != nil {
		t.Fatalf("expected cleared archive fields, got %+v", up)
	}
	if rec = sprintDurDo(t, rig, http.MethodPost, base+"/sprints", map[string]any{"name": "S2"}); rec.Code != http.StatusCreated {
		t.Fatalf("create after unarchive expected 201, got %d", rec.Code)
	}

	// Delete is still allowed on an archived project.
	sprintDurDo(t, rig, http.MethodPut, base+"/archive", nil)
	if rec = sprintDurDo(t, rig, http.MethodDelete, base, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete archived expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func decodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	_ = json.Unmarshal(rec.Body.Bytes(), &v)
	return v
}
