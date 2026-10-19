package auth

import "sync"

// Access is the live allowlist and admin list for one process.
// Sign-in and every authenticated request read it. Tests and operators
// replace the lists with Set. It is not a process-wide variable; pass the
// same pointer on Config.
type Access struct {
	mu      sync.RWMutex
	allowed []string
	admins  []string
}

// NewAccess copies the lists. Later Set calls replace them.
func NewAccess(allowed, admins []string) *Access {
	a := &Access{}
	a.Set(allowed, admins)
	return a
}

// Set replaces both lists. The slices are copied.
func (a *Access) Set(allowed, admins []string) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.allowed = append([]string(nil), allowed...)
	a.admins = append([]string(nil), admins...)
}

func (a *Access) role(email string) (string, bool) {
	if a == nil {
		return "", false
	}
	email = normalizeEmail(email)
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, e := range a.admins {
		if normalizeEmail(e) == email {
			return RoleAdmin, true
		}
	}
	for _, e := range a.allowed {
		if normalizeEmail(e) == email {
			return RoleLearner, true
		}
	}
	return "", false
}
