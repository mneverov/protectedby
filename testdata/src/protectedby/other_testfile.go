package protectedby

// funcInOtherFile is a function that is defined for the struct, declared in another file.
func (s *s1) funcInOtherFile() {
	s.protectedField3 = 42 // want `not protected access to shared field protectedField3, use s.mu.Lock()`
}
