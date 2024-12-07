package utils

import (
	"sync"

	"golang.org/x/exp/maps"
)

type ConcurrentSet[A comparable] struct {
	m map[A]*struct{}
	l sync.Mutex
}

func NewConcurrentSet[A comparable]() ConcurrentSet[A] {
	return ConcurrentSet[A]{
		m: map[A]*struct{}{},
	}
}

func (m *ConcurrentSet[A]) Set(k A) {
	m.l.Lock()
	defer m.l.Unlock()
	m.m[k] = &struct{}{}
}

func (m *ConcurrentSet[A]) Delete(k A) {
	m.l.Lock()
	defer m.l.Unlock()
	delete(m.m, k)
}

func (m *ConcurrentSet[A]) Exists(k A) bool {
	m.l.Lock()
	defer m.l.Unlock()
	_, exists := m.m[k]
	return exists
}

func (m *ConcurrentSet[A]) Values() []A {
	return maps.Keys(m.m)
}

func (m *ConcurrentSet[A]) Len() int {
	m.l.Lock()
	defer m.l.Unlock()
	return len(m.Values())
}

func (m *ConcurrentSet[A]) Reset() {
	m.l.Lock()
	defer m.l.Unlock()
	m.m = map[A]*struct{}{}
}
