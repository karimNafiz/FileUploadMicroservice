package safemap

// need to import the sync package
import (
	"sync"
)

// need to use generics
// the safe map can hold any key and any value

type safemap[V interface{}] struct {
	safemap map[string]V
	rw_lock sync.RWMutex
}

func NewSafeMap[V interface{}]() *safemap[V] {
	return &safemap[V]{
		safemap: make(map[string]V),
	}
}

func (m *safemap[V]) Add(key string, value V) {
	defer m.rw_lock.Unlock()
	m.rw_lock.Lock()
	m.safemap[key] = value

}

func (m *safemap[V]) Remove(key string) bool {
	if !m.Contains(key) {
		return false
	}
	defer m.rw_lock.Unlock()
	m.rw_lock.Lock()
	delete(m.safemap, key)
	return true
}

func (m *safemap[V]) Contains(key string) bool {
	defer m.rw_lock.RUnlock()
	m.rw_lock.RLock()
	_, exists := m.safemap[key]

	return exists
}

func (m *safemap[V]) Get(key string) (V, bool) {
	defer m.rw_lock.RUnlock()
	m.rw_lock.Lock()
	val, exists := m.safemap[key]
	return val, exists
}
