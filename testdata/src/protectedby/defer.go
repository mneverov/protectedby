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
		s.protectedField1 = 42 // want `not protected access to shared field protectedField1, use s.mu.Lock()`
	}()
}

func lockInDeferAccessInFunc() {
	s := s1{}
	defer s.mu.Lock()
	s.protectedField2 = 42 // want `not protected access to shared field protectedField2, use s.mu.Lock()`
}

func deferLockAfterAccessInDefer() {
	s := s1{}
	s.mu.Lock()

	defer func() {
		s.protectedField1 = 42
	}()

	defer s.mu.Unlock() // not reported. It is your fault if you do things like that!
}

func deferLockBeforeDeferAccess() {
	s := s1{}
	defer s.mu.Lock()

	defer func() {
		s.protectedField1 = 42 // want `not protected access to shared field protectedField1, use s.mu.Lock()`
	}()
}
