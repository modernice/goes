package workflow

import (
	"sync"

	"github.com/google/uuid"
)

// keyedMutex synchronizes work per key. Entries are reference-counted and
// removed once a key is neither held nor awaited, so the map does not grow
// with the number of workflows ever processed.
type keyedMutex struct {
	mu    sync.Mutex
	locks map[uuid.UUID]*keyedLock
}

type keyedLock struct {
	mu   sync.Mutex
	refs int
}

// Lock locks the mutex of the given key and returns the unlock function:
//
//	defer locks.Lock(id)()
func (km *keyedMutex) Lock(key uuid.UUID) (unlock func()) {
	km.mu.Lock()
	if km.locks == nil {
		km.locks = make(map[uuid.UUID]*keyedLock)
	}
	l := km.locks[key]
	if l == nil {
		l = &keyedLock{}
		km.locks[key] = l
	}
	l.refs++
	km.mu.Unlock()

	l.mu.Lock()

	return func() {
		l.mu.Unlock()

		km.mu.Lock()
		if l.refs--; l.refs == 0 {
			delete(km.locks, key)
		}
		km.mu.Unlock()
	}
}
