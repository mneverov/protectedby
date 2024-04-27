package protectedby

import "sync"

type inner struct {
	// i is protected by mu.
	i  int
	mu sync.Locker
}

type outer struct {
	n inner
}

func nestedAccess() {
	o := outer{}
	// todo(mneverov): access to a protected field of a nested struct is not reported.
	o.n.i = 42
}

func nestedFunction1() {
	s := inner{}
	f := func() {
		// Just don't do this otherwise you get a false positive warning.
		s.i = 42 // want `not protected access to shared field i, use s.mu.Lock()`
	}

	s.mu.Lock()

	f()
}

func nestedFunction2() {
	s := inner{}
	s.mu.Lock()

	func() {
		s.i = 42
	}()
}

func nestedFun() {
	f := inner{}

	Unlock := func() {
		f.mu.Unlock()
	}
	f.mu.Lock()
	Unlock()

	// Not protected access is not reported.
	f.i = 42
}
