package session

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const sessionTTL = 12 * time.Hour

type store struct {
	mu   sync.RWMutex
	data map[string]time.Time // token -> expiry
}

var global = &store{data: make(map[string]time.Time)}

// New создаёт новый сессионный токен и сохраняет его.
func New() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)

	global.mu.Lock()
	global.data[token] = time.Now().Add(sessionTTL)
	global.mu.Unlock()

	go global.cleanup()
	return token
}

// Valid проверяет токен.
func Valid(token string) bool {
	global.mu.RLock()
	exp, ok := global.data[token]
	global.mu.RUnlock()
	return ok && time.Now().Before(exp)
}

// Delete удаляет сессию (logout).
func Delete(token string) {
	global.mu.Lock()
	delete(global.data, token)
	global.mu.Unlock()
}

func (s *store) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for tok, exp := range s.data {
		if now.After(exp) {
			delete(s.data, tok)
		}
	}
}
