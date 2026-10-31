package protectedby

import "sync"

type lockNameStruct struct {
	// a is protected by muA.
	a   int
	muA sync.Mutex
	// b is protected by muB.
	b   int
	muB sync.Mutex
}

func lockOtherField() {
	s := lockNameStruct{}
	s.muB.Lock()
	defer s.muB.Unlock()

	s.a = 42 // want `not protected access to shared field a, use s.muA.Lock()`
}

func lockOtherThenOwnField() {
	s := lockNameStruct{}
	s.muB.Lock()
	s.muA.Lock()

	s.a = 42
}

func lockOwnThenOtherField() {
	s := lockNameStruct{}
	s.muA.Lock()
	s.muB.Lock()

	s.a = 42
}

func unlockOtherField() {
	s := lockNameStruct{}
	s.muA.Lock()
	s.muB.Lock()
	s.muB.Unlock()

	s.a = 42
}

func unlockOwnField() {
	s := lockNameStruct{}
	s.muA.Lock()
	s.muB.Lock()
	s.muA.Unlock()

	s.a = 42 // want `not protected access to shared field a, use s.muA.Lock()`
}

func bothFieldsBothLocks() {
	s := lockNameStruct{}
	s.muA.Lock()
	defer s.muA.Unlock()
	s.muB.Lock()
	defer s.muB.Unlock()

	s.a = 42
	s.b = 42
}

func relockAfterUnlock() {
	s := lockNameStruct{}
	s.muA.Lock()
	s.muA.Unlock()
	s.muA.Lock()

	s.a = 42
}

func unlockThenDeferredUnlock() {
	s := lockNameStruct{}
	s.muA.Lock()
	s.muA.Unlock()
	defer func() {
		s.muA.Unlock()
	}()

	s.a = 42 // want `not protected access to shared field a, use s.muA.Lock()`
}
