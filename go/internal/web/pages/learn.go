package pages

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/csrf"
	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/path"
	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/views/components"
	learnview "github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/views/pages"
)

// Learning serves Today and the path map from a Reader and a GateAck.
// The dev mount passes fixtures. Sessions are not here yet, so the reader
// id is path.DevUserID.
type Learning struct {
	reader path.Reader
	gate   path.GateAck
	cfg    path.Config
	planID string
}

// NewLearning returns the Today and path handler.
// planID is the published plan id passed to Reader.Path.
func NewLearning(reader path.Reader, gate path.GateAck, cfg path.Config, planID string) *Learning {
	return &Learning{reader: reader, gate: gate, cfg: cfg, planID: planID}
}

// Register mounts Today, the path, the gate acknowledgement, and htmx.
// Patterns are exact so /lessons/ does not swallow /lessons/all.
func (h *Learning) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", h.Today)
	mux.HandleFunc("GET /lessons", h.Lessons)
	mux.HandleFunc("GET /lessons/{$}", h.Lessons)
	mux.HandleFunc("POST /_hx/units/{id}/gate-ack", h.GateAck)
	mux.HandleFunc("GET /_static/htmx.min.js", htmx)
}

// Today is GET /.
func (h *Learning) Today(w http.ResponseWriter, r *http.Request) {
	page := h.load(w, r)
	writeHTML(w, r, http.StatusOK, learnview.TodayPage(page), learnview.TodayFragment(page))
}

// Lessons is GET /lessons/.
func (h *Learning) Lessons(w http.ResponseWriter, r *http.Request) {
	page := h.load(w, r)
	writeHTML(w, r, http.StatusOK, learnview.LessonsPage(page), learnview.LessonsFragment(page))
}

func (h *Learning) load(w http.ResponseWriter, r *http.Request) learnview.LearnPage {
	page := learnview.LearnPage{CSRF: csrf.Issue(w, r)}
	snap, err := h.reader.Path(r.Context(), path.DevUserID, h.planID)
	if err != nil {
		page.Failed = true
		return page
	}
	due, dueErr := h.reader.DueCount(r.Context(), path.DevUserID)
	if dueErr != nil {
		snap.DueKnown = false
	} else {
		snap.Due = due
		snap.DueKnown = true
	}
	page.Snap = path.Present(h.cfg, snap)
	return page
}

// GateAck is POST /_hx/units/{id}/gate-ack.
// A failed write, including ErrNotPersisted, still returns the warning
// with working links. Origin or token failures are 403.
func (h *Learning) GateAck(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	if err := csrf.Check(r); err != nil {
		forbid(w)
		return
	}
	id := r.PathValue("id")
	_ = h.gate.MarkGateWarningSeen(r.Context(), path.DevUserID, h.planID, id)
	next := safeLocal(r.FormValue("next"))
	checkpoint := safeLocal(r.FormValue("checkpoint"))
	dialog := components.GateDialog{UnitID: id, Next: next, Checkpoint: checkpoint}
	snap, err := h.reader.Path(r.Context(), path.DevUserID, h.planID)
	if err == nil {
		if n, ok := path.UnfinishedUnit(snap, id); ok {
			dialog.Known = true
			dialog.TitleEn = "You haven't finished Unit " + itoa(n) + "."
			dialog.TitleHe = "עוד לא סיימת את יחידה " + itoa(n) + "."
		}
	}
	hdr := w.Header()
	hdr.Set("Content-Type", "text/html; charset=utf-8")
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Referrer-Policy", "same-origin")
	hdr.Set("X-Frame-Options", "DENY")
	hdr.Set("Cache-Control", "no-store")
	_ = components.GateSheet(dialog).Render(r.Context(), w)
}

func forbid(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/plain; charset=utf-8")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte("forbidden\n"))
}

var localPath = regexp.MustCompile(`^/[\w./-]*$`)

func safeLocal(raw string) string {
	if strings.Contains(raw, "..") || !localPath.MatchString(raw) {
		return ""
	}
	return raw
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
