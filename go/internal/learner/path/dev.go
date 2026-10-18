package path

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

// EnvDevWeb is the environment variable that turns on the read-only JSON
// routes. The value must be exactly "1". Any other value, including empty,
// leaves the routes unregistered.
const EnvDevWeb = "HEBREW_DEV_WEB"

// MountDev registers GET /api/dev/learner/due and GET /api/dev/learner/path
// when enabled is true. The routes read the service and do not write.
// enabled false registers nothing.
//
//	GET /api/dev/learner/due?user=<id>
//	GET /api/dev/learner/path?user=<id>&plan=<id>
//
// plan is optional. An empty plan selects the assigned plan.
func MountDev(mux *http.ServeMux, svc *Service, enabled bool) {
	if mux == nil || !enabled {
		return
	}
	mux.HandleFunc("GET /api/dev/learner/due", svc.devDue)
	mux.HandleFunc("GET /api/dev/learner/path", svc.devPath)
}

func (s *Service) devDue(w http.ResponseWriter, r *http.Request) {
	userID, ok := devUser(w, r)
	if !ok {
		return
	}
	n, err := s.DueCount(r.Context(), userID)
	if err != nil {
		devErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		UserID   int64 `json:"userId"`
		DueCount int   `json:"dueCount"`
	}{UserID: userID, DueCount: n})
}

func (s *Service) devPath(w http.ResponseWriter, r *http.Request) {
	userID, ok := devUser(w, r)
	if !ok {
		return
	}
	got, err := s.Path(r.Context(), userID, r.URL.Query().Get("plan"))
	if err != nil {
		devErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, got)
}

func devUser(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := r.URL.Query().Get("user")
	if raw == "" {
		writeJSON(w, http.StatusBadRequest, devError{Error: "bad_request"})
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, devError{Error: "bad_request"})
		return 0, false
	}
	return id, true
}

type devError struct {
	Error string `json:"error"`
}

func devErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrBadUser):
		writeJSON(w, http.StatusBadRequest, devError{Error: "bad_request"})
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrNoPlan):
		writeJSON(w, http.StatusNotFound, devError{Error: "not_found"})
	default:
		writeJSON(w, http.StatusInternalServerError, devError{Error: "error"})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
