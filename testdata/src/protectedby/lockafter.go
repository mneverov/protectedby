package protectedby

func lockAfterAccess() {
	s := s1{}
	s.protectedField1 = 42 // todo(mneverov): want ...
	s.mu.Lock()
}
