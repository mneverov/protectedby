package protectedby

/*
This is added for completeness. I can't think of use cases when locking in defer is OK.
*/

func protectedAccessInDefer() {
	s := s1{}
	defer func() {
		s.mu.Lock()
		s.protectedField1 = 42
	}()
}

func notProtectedAccessInDefer() {
	s := s1{}
	defer func() {
		s.protectedField1 = 42 // todo(mneverov): want ...
	}()
}

func lockInDeferAccessInFunc() {
	s := s1{}
	defer s.mu.Lock()
	s.protectedField1 = 42 // todo(mneverov): want ...
}

func accessInDeferLockInFunc() {
	s := s1{}
	s.mu.Lock()
	defer s.mu.Unlock()

	defer func() {
		s.protectedField1 = 42 // todo(mneverov): want ...
	}()
}
