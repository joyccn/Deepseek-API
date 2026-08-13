package agentic

import (
	"sync"
)

type SessionCache struct {
	cache sync.Map
}

func NewSessionCache() *SessionCache {
	return &SessionCache{}
}

func (s *SessionCache) Get(key string) (string, bool) {
	val, ok := s.cache.Load(key)
	if !ok {
		return "", false
	}
	return val.(string), true
}

func (s *SessionCache) Set(key, sessionID string) {
	s.cache.Store(key, sessionID)
}

func (s *SessionCache) Delete(key string) {
	s.cache.Delete(key)
}
