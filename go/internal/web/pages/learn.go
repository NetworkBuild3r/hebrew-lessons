package pages

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/csrf"
	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/learner/config"
	lpath "github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/learner/path"
	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/path"
	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/views/components"
	learnview "github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/views/pages"
)

// Learning serves Today and the path map from path.New(db, cfg).
// Sessions are not here yet, so the reader id is lpath.DevUserID.
// gate is path.Unavailable until unit_progress exists.
type Learning struct {
	svc    *lpath.Service
	gate   lpath.GateAck
	cfg    config.Config
	planID string
}

// NewLearning returns the Today and path handler.
// planID is the published plan id passed to Service.Path.
func NewLearning(svc *lpath.Service, gate lpath.GateAck, cfg config.Config, planID string) *Learning {
	return &Learning{svc: svc, gate: gate, cfg: cfg, planID: planID}
}

// Register mounts Today, the path, and the gate acknowledgement.
// htmx is the file PR #34 vendors at /shared/vendor/htmx.min.js.
// Patterns are exact so /lessons/ does not swallow /lessons/all.
func (h *Learning) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", h.Today)
	mux.HandleFunc("GET /lessons", h.Lessons)
	mux.HandleFunc("GET /lessons/{$}", h.Lessons)
	mux.HandleFunc("POST /_hx/units/{id}/gate-ack", h.GateAck)
}

// Today is GET /.
func (h *Learning) Today(w http.ResponseWriter, r *http.Request) {
	page := h.load(w, r)
	page.Swapped = r.Header.Get("HX-Request") == "true"
	writeHTML(w, r, http.StatusOK, learnview.TodayPage(page), learnview.TodayFragment(page))
}

// Lessons is GET /lessons/.
func (h *Learning) Lessons(w http.ResponseWriter, r *http.Request) {
	page := h.load(w, r)
	page.Swapped = r.Header.Get("HX-Request") == "true"
	writeHTML(w, r, http.StatusOK, learnview.LessonsPage(page), learnview.LessonsFragment(page))
}

func (h *Learning) load(w http.ResponseWriter, r *http.Request) learnview.LearnPage {
	page := learnview.LearnPage{CSRF: csrf.Issue(w, r)}
	course, err := h.svc.Path(r.Context(), lpath.DevUserID, h.planID)
	if err != nil {
		page.Failed = true
		return page
	}
	due, dueErr := h.svc.DueCount(r.Context(), lpath.DevUserID)
	dueKnown := dueErr == nil
	if !dueKnown {
		due = 0
	}
	page.Snap = path.Present(h.cfg, course, due, dueKnown)
	h.applySeen(r, &page.Snap)
	return page
}

// applySeen opens a unit's links when GateWarningSeen is stored.
// An error, including ErrNotPersisted, leaves the sheet in place.
func (h *Learning) applySeen(r *http.Request, snap *path.Snapshot) {
	openUnit := func(nodes []path.Node, unitID string) []path.Node {
		seen, err := h.gate.GateWarningSeen(r.Context(), lpath.DevUserID, h.planID, unitID)
		if err != nil || !seen {
			return nodes
		}
		for i := range nodes {
			if nodes[i].UnitID == unitID && nodes[i].Gate == "warn" {
				nodes[i].Gate = "open"
				nodes[i].HXVals = ""
			}
		}
		return nodes
	}
	for i := range snap.Units {
		snap.Units[i].Nodes = openUnit(snap.Units[i].Nodes, snap.Units[i].ID)
	}
	if snap.Rail.UnitID != "" {
		snap.Strip = openUnit(snap.Strip, snap.Rail.UnitID)
	}
}

// GateAck is POST /_hx/units/{id}/gate-ack.
// confirm=1 records the ack (Go anyway / Test out) and returns 204.
// The opening post does not record it. It returns the sheet and flips
// that unit's links to data-gate="open" for the rest of this page view.
// A failed write, including ErrNotPersisted, still returns the sheet
// with working links. Origin or token failures are 403.
func (h *Learning) GateAck(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	if err := csrf.Check(r); err != nil {
		forbid(w)
		return
	}
	id := r.PathValue("id")
	if r.FormValue("confirm") == "1" {
		_ = h.gate.MarkGateWarningSeen(r.Context(), lpath.DevUserID, h.planID, id)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	next := safeLocal(r.FormValue("next"))
	checkpoint := safeLocal(r.FormValue("checkpoint"))
	dialog := components.GateDialog{
		UnitID: id, Next: next, Checkpoint: checkpoint,
		TriggerID: r.FormValue("trigger"),
	}
	var flipped []path.Node
	course, err := h.svc.Path(r.Context(), lpath.DevUserID, h.planID)
	if err == nil {
		if n, ok := path.UnfinishedUnit(course, id); ok {
			dialog.Known = true
			dialog.TitleEn = "You haven't finished Unit " + itoa(n) + "."
			dialog.TitleHe = "עוד לא סיימת את יחידה " + itoa(n) + "."
		}
		due, dueErr := h.svc.DueCount(r.Context(), lpath.DevUserID)
		snap := path.Present(h.cfg, course, due, dueErr == nil)
		flipped = openUnitNodes(snap, id)
	}
	hdr := w.Header()
	hdr.Set("Content-Type", "text/html; charset=utf-8")
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Referrer-Policy", "same-origin")
	hdr.Set("X-Frame-Options", "DENY")
	hdr.Set("Cache-Control", "no-store")
	_ = components.GateAckBody(dialog, flipped).Render(r.Context(), w)
}

func openUnitNodes(snap path.Snapshot, unitID string) []path.Node {
	var out []path.Node
	for _, u := range snap.Units {
		if u.ID != unitID {
			continue
		}
		for _, n := range u.Nodes {
			if n.Gate != "warn" {
				continue
			}
			n.Gate = "open"
			n.HXVals = ""
			out = append(out, n)
		}
	}
	return out
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
	if strings.Contains(raw, "..") || strings.Contains(raw, "//") || !localPath.MatchString(raw) {
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
