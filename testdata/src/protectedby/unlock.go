package protectedby

import "sync"

type unlockStruct struct {
	// protected by mu
	i  *int
	mu sync.Mutex
}

func unlockAfterLock(i *int) {
	s := unlockStruct{}
	s.mu.Lock()
	s.mu.Unlock()

	s.i = i // want `not protected access to shared field i, use s.mu.Lock()`
}

func unlockBeforeLock(i *int) {
	s := unlockStruct{}
	s.mu.Unlock()
	s.mu.Lock()

	s.i = i
}

func deferUnlockIsFine(i *int) {
	s := unlockStruct{}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.i = i
}
