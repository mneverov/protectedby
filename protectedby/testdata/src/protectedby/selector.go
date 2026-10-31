package protectedby

import "sync"

type selectorStruct struct {
	// c is protected by mu.
	c  counter
	mu sync.Mutex
}

// The protected field is the inner selector of s.c.v and s.c.get().
func accessThroughSelector() int {
	s := selectorStruct{}
	s.c.v = 1        // want `not protected access to shared field c, use s.mu.Lock()`
	return s.c.get() // want `not protected access to shared field c, use s.mu.Lock()`
}

func protectedAccessThroughSelector() int {
	s := selectorStruct{}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.c.inc()
	return s.c.get()
}

// selfProtected mirrors go/token.File: the comment is attached to the lock, not to the fields it guards.
type selfProtected struct {
	// lines and infos are protected by mu.
	mu    sync.Mutex
	lines []int
}

func lockSelfProtected() int {
	s := selfProtected{}
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.lines)
}
