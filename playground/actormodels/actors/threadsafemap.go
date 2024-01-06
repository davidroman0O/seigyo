package actors

import "sync"

// ThreadSafeMap is a struct that holds a generic map and a mutex for synchronization.
type ThreadSafeMap[Key ~string | ~int, Value any] struct {
	sync.RWMutex
	items map[Key]Value
	keys  map[Key]struct{} // Use a map for tracking keys
}

// NewThreadSafeMap creates a new instance of a thread-safe map.
func NewThreadSafeMap[Key ~string | ~int, Value any]() *ThreadSafeMap[Key, Value] {
	return &ThreadSafeMap[Key, Value]{
		items: make(map[Key]Value),
		keys:  make(map[Key]struct{}),
	}
}

// Set adds or updates an element in the map.
func (m *ThreadSafeMap[Key, Value]) Set(key Key, value Value) {
	m.Lock()
	defer m.Unlock()
	m.items[key] = value
	m.keys[key] = struct{}{} // Add key to keys map
}

// Get retrieves an element from the map.
func (m *ThreadSafeMap[Key, Value]) Get(key Key) (Value, bool) {
	m.RLock()
	defer m.RUnlock()
	val, ok := m.items[key]
	return val, ok
}

// Delete removes an element from the map.
func (m *ThreadSafeMap[Key, Value]) Delete(key Key) {
	m.Lock()
	defer m.Unlock()
	delete(m.items, key)
	delete(m.keys, key) // Remove key from keys map
}

// Has checks if the given key exists in the map.
func (m *ThreadSafeMap[Key, Value]) Has(key Key) bool {
	m.RLock()
	defer m.RUnlock()
	_, ok := m.keys[key]
	return ok
}

func (m *ThreadSafeMap[Key, Value]) Keys() []Key {
	m.RLock()
	defer m.RUnlock()
	keys := make([]Key, 0, len(m.keys))
	for key := range m.keys {
		keys = append(keys, key)
	}
	return keys // Return a slice of keys
}
