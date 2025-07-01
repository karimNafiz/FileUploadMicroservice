package safemap

// TODO need to fix a lock
// or else both readers and writers will get locked
// go through the code again

// need to import the sync package
import (
	"sync"
)

// need to use generics
// the safe map can hold any key and any value

type SafeMap[V interface{}] struct {
	SafeMap map[string]V
	rw_lock sync.RWMutex
}

func NewSafeMap[V interface{}]() *SafeMap[V] {
	return &SafeMap[V]{
		SafeMap: make(map[string]V),
	}
}

func (m *SafeMap[V]) Add(key string, value V) {
	defer m.rw_lock.Unlock()
	m.rw_lock.Lock()
	m.SafeMap[key] = value

}

func (m *SafeMap[V]) Remove(key string) bool {
	if !m.Contains(key) {
		return false
	}
	defer m.rw_lock.Unlock()
	m.rw_lock.Lock()
	delete(m.SafeMap, key)
	return true
}

func (m *SafeMap[V]) Contains(key string) bool {
	defer m.rw_lock.RUnlock()
	m.rw_lock.RLock()
	_, exists := m.SafeMap[key]

	return exists
}

func (m *SafeMap[V]) Get(key string) (V, bool) {
	defer m.rw_lock.RUnlock()
	m.rw_lock.RLock()
	val, exists := m.SafeMap[key]
	return val, exists
}
