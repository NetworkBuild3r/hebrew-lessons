package mcp

import (
	"regexp"
	"time"
)

var secretRE = regexp.MustCompile(`sk-[A-Za-z0-9_-]{8,}`)

const mcpReadsPerHour = 60

// AuditWriter appends one audit row. The read-only database cannot do this.
// A nil writer makes get_learner_profile and check_text_coverage refuse
// before they read anything, so the audit is not silently skipped.
type AuditWriter interface {
	Append(actor, action, target, detail string, at time.Time) error
}

// Learner is the account a profile or coverage read is about.
type Learner struct {
	ID         int64
	Username   string
	AIPlanning bool
	Active     bool
}

// LearnerReader loads a profile or a coverage result. It is unused while
// AuditWriter is nil.
type LearnerReader interface {
	Find(username string) (Learner, bool, error)
	Profile(username string) (Value, error)
	Coverage(username, text string) (Value, error)
}

type readWindow struct {
	hour  int64
	count int
}

func (a *App) admit(username, action, invalid string) (string, Learner, error) {
	var learner Learner
	var found bool
	var err error
	if a.learner != nil {
		learner, found, err = a.learner.Find(username)
		if err != nil {
			return "", Learner{}, err
		}
	}
	active := found && learner.Active
	now := a.now()
	outcome := ""
	if !a.takeRead(now) {
		outcome = "rate_limited"
	} else if invalid != "" {
		outcome = invalid
	} else if !active {
		outcome = "not_found"
	} else if !learner.AIPlanning {
		outcome = "planning_off"
	}
	target := ""
	if found {
		target = learner.Username
	}
	if outcome != "" {
		detail := `{"outcome":"` + outcome + `","fields":"none"}`
		if err := a.record(action, target, detail); err != nil {
			return "", Learner{}, err
		}
		return outcome, Learner{}, nil
	}
	return "", learner, nil
}

func (a *App) takeRead(now time.Time) bool {
	a.readsMu.Lock()
	defer a.readsMu.Unlock()
	hour := now.UTC().Unix() / 3600
	if a.reads == nil || hour > a.reads.hour {
		a.reads = &readWindow{hour: hour, count: 1}
		return true
	}
	if a.reads.count >= mcpReadsPerHour {
		return false
	}
	a.reads.count++
	return true
}

func (a *App) record(action, username, detail string) error {
	if a.audit == nil {
		return errNoAudit
	}
	target := username
	return a.audit.Append("mcp", action, target, detail, a.now())
}

func (a *App) now() time.Time {
	if a.clock != nil {
		return a.clock()
	}
	return time.Now()
}

type errString string

func (e errString) Error() string { return string(e) }

const errNoAudit = errString("audit writer is not configured")
