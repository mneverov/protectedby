package protectedby

import "sync"

type s5 struct {
	// field is protected by muInt. Not really since mu does not implement sync.Locker.
	field int
	muInt int // want `lock muInt doesn't implement sync.Locker interface`
}

type lockerAlias sync.Locker

type s6 struct {
	// s6f is protected by mu.
	s6f int
	mu  lockerAlias
}

// myLocker implements sync.Locker. The receiver for the functions is not a pointer on purpose.
type myLocker struct{}

func (ml myLocker) Lock()   {}
func (ml myLocker) Unlock() {}

type s7 struct {
	// s7f is protected by mu
	s7f int
	mu  myLocker
}
