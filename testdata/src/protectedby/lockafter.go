package protectedby

func lockAfterAccess() {
	s := s1{}
	s.protectedField1 = 42 // want `not protected access to shared field protectedField1, use s.mu.Lock()`
	s.mu.Lock()
}
