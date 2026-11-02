package protectedby

import "sync"

// The analyzer compares positions, not control flow: Lock() must appear before the access in the same function,
// with no Unlock() between them. Branches, loops, and early returns are not modeled.
type controlFlowStruct struct {
	// n is protected by mu.
	n  int
	mu sync.Mutex
}

// False positive: the Unlock() is followed by a return, so mu is held at the access.
func earlyUnlock(c bool) {
	s := controlFlowStruct{}
	s.mu.Lock()
	if c {
		s.mu.Unlock()
		return
	}
	s.n = 1 // want `not protected access to shared field n, use s.mu.Lock()`
	s.mu.Unlock()
}

// False negative: mu is held at the access only if c is true.
func conditionalLock(c bool) {
	s := controlFlowStruct{}
	if c {
		s.mu.Lock()
	}
	s.n = 1
}
