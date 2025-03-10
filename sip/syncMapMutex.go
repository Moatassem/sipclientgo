package sip

import (
	"maps"
	"sync"
)

type ConcurrentMapMutex[T any] struct {
	datamp map[string]*T
	mu     sync.RWMutex
}

func NewConcurrentMapMutex[T any]() *ConcurrentMapMutex[T] {
	return &ConcurrentMapMutex[T]{datamp: make(map[string]*T)}
}

func (c *ConcurrentMapMutex[T]) Store(ky string, ss *T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.datamp[ky] = ss
}

func (c *ConcurrentMapMutex[T]) Delete(ky string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.datamp, ky)
}

func (c *ConcurrentMapMutex[T]) Load(ky string) (*T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.datamp[ky]
	return s, ok
}

func (c *ConcurrentMapMutex[T]) Range() map[string]*T {
	c.mu.RLock()
	defer c.mu.RUnlock()
	copy := make(map[string]*T, len(c.datamp))
	maps.Copy(copy, c.datamp)
	return copy
}

func (c *ConcurrentMapMutex[T]) IsEmpty() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.datamp) == 0
}
