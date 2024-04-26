package protectedby

import "sync"

func wrongLock() {
	s := s1{}
	mu := sync.Mutex{}
	mu.Lock() // not related lock

	s.protectedField1 = 42 // want `not protected access to shared field protectedField1, use s.mu.Lock()`
}
