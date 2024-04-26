package protectedby

import "sync"

type wrongLockStruct struct {
	// i is protected by mu.
	i  int
	mu sync.Mutex
}

func wrongLock() {
	s := wrongLockStruct{}
	mu := sync.Mutex{}
	mu.Lock() // not related lock

	s.i = 42 // want `not protected access to shared field i, use s.mu.Lock()`
}
