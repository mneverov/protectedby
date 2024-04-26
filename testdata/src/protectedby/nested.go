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
	o.n.i = 42 //  todo(mneverov): want ...
}

func nestedFunction1() {
	s := inner{}
	f := func() {
		// just don't do this otherwise you get a false positive warning
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
