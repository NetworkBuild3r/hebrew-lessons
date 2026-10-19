package activity

import (
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// tokenKeyInfo is the HKDF info label of the item token key. HEBREW_SESSION_SECRET is the
// input keying material, the same secret the OAuth state key comes from, with its own label
// so one key can never stand in for the other.
const tokenKeyInfo = "hebrew-activity-item-token-v1"

// KeyFromSecret derives the 32-byte HMAC key of item tokens from HEBREW_SESSION_SECRET.
// An empty secret gives a random key, which lasts for the life of the process. Tokens then
// stop working after a restart, which is when the pending items are lost anyway.
func KeyFromSecret(secret string) ([]byte, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		return key, nil
	}
	return hkdf.Key(sha256.New, []byte(secret), nil, tokenKeyInfo, 32)
}

// claims is what a token binds. It is signed, not secret, and it holds no answer.
type claims struct {
	ID     string `json:"i"`
	User   int64  `json:"u"`
	Lexeme int64  `json:"l"`
	Kind   Kind   `json:"a"`
	Exp    int64  `json:"e"`
}

func signClaims(key []byte, c claims) (string, error) {
	body, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(body)
	enc := base64.RawURLEncoding
	return enc.EncodeToString(body) + "." + enc.EncodeToString(mac.Sum(nil)), nil
}

// verifyClaims checks the signature and returns the claims. It does not check the expiry
// or the learner: Check does, so each failure has its own error.
func verifyClaims(key []byte, token string) (claims, error) {
	body64, sig64, ok := strings.Cut(token, ".")
	if !ok || len(token) > 1024 {
		return claims{}, ErrTokenInvalid
	}
	enc := base64.RawURLEncoding
	body, err := enc.DecodeString(body64)
	if err != nil {
		return claims{}, ErrTokenInvalid
	}
	sig, err := enc.DecodeString(sig64)
	if err != nil {
		return claims{}, ErrTokenInvalid
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(body)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return claims{}, ErrTokenInvalid
	}
	var c claims
	if err := json.Unmarshal(body, &c); err != nil || c.ID == "" {
		return claims{}, ErrTokenInvalid
	}
	return c, nil
}

func newNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// pending is the answer the server keeps for one built item. The client never sees it.
type pending struct {
	user, lexeme int64
	kind         Kind
	skill        string
	source       string
	unit         int64
	builtAt      time.Time
	expires      time.Time

	// Words, one per option, or one per left entry of a match item.
	words []Word
	// For a pick item: the index of the right option. words[i] is option i.
	correct int
	// For a match item: rightWords[j] is the word whose gloss is Right[j], and
	// rightOf[i] is the index into Right that is right for Left[i]. words[i] is Left[i].
	rightWords []Word
	rightOf    []int
	// For a typed item: the pointed text expected.
	expected string

	// claimed is true while Check writes. wrote counts the attempts already stored, so a
	// retry after a failed write resumes instead of writing twice.
	claimed bool
	wrote   int
	ratings []fsrs.Rating
}

// pendingSet holds the unanswered items. It is bounded: past MaxPending the oldest go.
type pendingSet struct {
	mu    sync.Mutex
	items map[string]*pending
	order []string
}

func newPendingSet() *pendingSet { return &pendingSet{items: map[string]*pending{}} }

func (s *pendingSet) add(id string, p *pending, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for len(s.items) >= MaxPending && len(s.order) > 0 {
		old := s.order[0]
		s.order = s.order[1:]
		delete(s.items, old)
	}
	// Expired items are dropped as they reach the front of the queue.
	for len(s.order) > 0 {
		front, ok := s.items[s.order[0]]
		if ok && front.expires.After(now) {
			break
		}
		delete(s.items, s.order[0])
		s.order = s.order[1:]
	}
	s.items[id] = p
	s.order = append(s.order, id)
}

func (s *pendingSet) remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, id)
}

// claim returns the pending item and marks it in use. ok is false when the item is gone or
// another Check holds it.
func (s *pendingSet) claim(id string) (*pending, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.items[id]
	if !ok || p.claimed {
		return nil, false
	}
	p.claimed = true
	return p, true
}

func (s *pendingSet) release(p *pending) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.claimed = false
}

func (s *pendingSet) size() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.items)
}
