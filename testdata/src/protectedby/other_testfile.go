package protectedby

import . "sync"

// funcInOtherFile is a function that is defined for the struct, declared in another file.
func (s *s1) funcInOtherFile() {
	s.protectedField3 = 42 // todo(mneverov): want this to fail
}

// s4 is a struct in another file in the same package with a protected field.
type s4 struct {
	// s4protectedField protected by s4mu.
	s4protectedField int
	// s4mu protects s4protectedField.
	s4mu Mutex
}
