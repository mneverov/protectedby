package protectedby

import (
	"fmt"
	"sync"
)

// This comment is associated with the hello constant. It cannot be protected by anything even if it declares that it
// is protected by something.
const hello = "Hello, World!"

// This comment is associated with the foo variable. It cannot be protected by anything even if it declares that it is
// protected by something.
var foo = hello

// main is a main function. It cannot be protected by anything even if it declares that it is protected by something.
func main() {
	fmt.Println(hello)
}

// s1 is a struct with some protected fields.
type s1 struct {
	// protectedField1 is
	// protected by mu.
	protectedField1 int
	protectedField2 int // protectedField2 is protected by mu.
	/*
		protectedField3 is
		protected by mu
		and also has a multiline comment.
	*/
	protectedField3 int

	// protectedField4 has a comment with empty line between the comment and field declaration.
	// protected by mu.

	protectedField4 int

	// protectedField5 is protected by mu.
	protectedField5 int
	// protectedField6 is protected by "mu". It is ok to put the lock name in parenthesis.
	protectedField6 int
	// protectedField7 is some shared resource. Protected by mu. Pattern is compared ignoring case.
	protectedField7 *int
	// field1 is protected by: mu. It is NOT checked by the linter because of the semicolon.
	field1 int
	// field2 is protected by mu and protected by mu.// want `found 2 "protected by " in comment "// field2 is protected by mu and protected by mu.", expected exact one`
	field2 int

	// field3 is protected by not existing mutex.// want `struct "s1" does not have lock field "not"`
	field3 int

	mu sync.Mutex

	// comment that does not belong to any field
	// but still protected by mu. Not checked.
}

// func1 demonstrates not protected write access.
func (s *s1) func1() {
	s.protectedField1 = 42 // want `not protected access to shared field protectedField1, use s.mu.Lock()`
}

// func2 demonstrates protected write access.
func (s *s1) func2() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.protectedField2 = 42 // the access is protected, all is fine.
	_ = s.protectedField7
}

// func3 demonstrates not protected read access.
func (s *s1) func3() {
	_ = s.protectedField3    // want `not protected access to shared field protectedField3, use s.mu.Lock()`
	tmp := s.protectedField4 // want `not protected access to shared field protectedField4, use s.mu.Lock()`
	fmt.Println(tmp)
	fmt.Println(s.protectedField5) // want `not protected access to shared field protectedField5, use s.mu.Lock()`
	if s.protectedField6 > 0 {     // want `not protected access to shared field protectedField6, use s.mu.Lock()`
		if 42 > s.protectedField6 { // want `not protected access to shared field protectedField6, use s.mu.Lock()`
			// nothing interesting here
		}
	}
}

// s2 is another struct with a protected field.
type s2 struct {
	// s2protectedField protected by s2mu.
	s2protectedField int
	// s2mu protects s2protectedField.
	s2mu sync.Mutex
}

func (s *s2) wantError() {
	s.s2protectedField = 42 // want `not protected access to shared field s2protectedField, use s.s2mu.Lock()`
}

// weird comment outside struct that does not belong to anything
// but still declares that it is protected by something.
