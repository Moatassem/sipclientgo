package sip

import (
	"sipclientgo/global"
	"sync"
)

type ConcurrentMapMutex struct {
	_map map[string]*SipSession
	mu   sync.RWMutex
}

func NewConcurrentMapMutex() ConcurrentMapMutex {
	return ConcurrentMapMutex{_map: make(map[string]*SipSession)}
}

func (c *ConcurrentMapMutex) Store(ky string, ss *SipSession) (ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ok = true
	if ss.Direction == global.INBOUND {
		ok = false
	}
	c._map[ky] = ss
	return
}

func (c *ConcurrentMapMutex) Delete(ky string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c._map, ky)
}

func (c *ConcurrentMapMutex) Load(ky string) (*SipSession, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c._map[ky]
	return s, ok
}

func (c *ConcurrentMapMutex) Range() map[string]*SipSession {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c._map
}

func (c *ConcurrentMapMutex) IsEmpty() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c._map) == 0
}
