package protectedby

// funcInOtherFile is a function that is defined for the struct, declared in another file.
func (s *s1) funcInOtherFile() {
	s.field5 = 42
}
