package protectedby

import "sync"

// The lock fields are declared with several names in one field.
type multiNameLock struct {
	// a is protected by muA.
	a int
	// b is protected by muB.
	b        int
	muA, muB sync.Mutex
}

func lockMultiName() {
	s := multiNameLock{}
	s.muA.Lock()
	defer s.muA.Unlock()

	s.a = 1
	s.b = 1 // want `not protected access to shared field b, use s.muB.Lock\(\)`
}

func noLockMultiName() {
	s := multiNameLock{}
	s.a = 1 // want `not protected access to shared field a, use s.muA.Lock\(\)`
}
