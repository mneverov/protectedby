package protectedby

import "sync"

/*
This is added for completeness. I can't think of use cases when locking in defer is OK.
*/

type deferLockStruct struct {
	// i is protected by mu.
	i  int
	mu sync.Mutex
}

func protectedAccessInDefer() {
	s := deferLockStruct{}

	defer func() {
		s.mu.Lock()
		s.i = 42
	}()
}

func notProtectedAccessInDefer() {
	s := deferLockStruct{}

	defer func() {
		s.i = 42 // want `not protected access to shared field i, use s.mu.Lock()`
	}()
}

func lockInDeferAccessInFunc() {
	s := deferLockStruct{}
	defer s.mu.Lock()

	s.i = 42 // want `not protected access to shared field i, use s.mu.Lock()`
}

func deferAccessAfterLockInFunc() {
	s := deferLockStruct{}
	s.mu.Lock()

	defer func() {
		s.i = 42
	}()

	// False negative (NOT reported). Will be unlocked before the protected field access.
	// It is your fault if you do things like that!
	defer s.mu.Unlock()
}

func lockAndAccessInDifferentDeferFunctions() {
	s := deferLockStruct{}
	defer s.mu.Lock()

	defer func() {
		s.i = 42 // want `not protected access to shared field i, use s.mu.Lock()`
	}()
}
