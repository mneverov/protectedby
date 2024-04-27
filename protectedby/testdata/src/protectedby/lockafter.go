package protectedby

import "sync"

type lockAfter struct {
	// i is protected by mu.
	i  int
	mu sync.Mutex
}

func lockAfterAccess() {
	s := lockAfter{}
	s.i = 42 // want `not protected access to shared field i, use s.mu.Lock()`
	s.mu.Lock()
}
