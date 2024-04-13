package protectedby

import (
	"fmt"
	"sync"
)

// This comment is associated with the hello constant. It cannot be protected by anything.
const hello = "Hello, World!" // line comment 1

// This comment is associated with the foo variable. It cannot be protected by anything.
var foo = hello // line comment 2

// main is a main function. It cannot be protected by anything.
func main() {
	fmt.Println(hello) // line comment 3
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
	protectedField7 int
	// field1 is protected by: mu. It is not checked by the linter because
	// of the semicolon. The pattern is "protected by <lock_name>" -- with a space between.
	field1 int
	// field2 is protected by mu and protected by mu.// want "found 2 \"protected by \" in comment \"// field8 is protected by mu and protected by mu.\", expected exact one"
	field2 int

	// field3 is protected by not existing mutex.// want `struct "s1" does not have lock field "not"`
	field3 int

	mu sync.Mutex

	// comment that does not belong to any field
	// but still protected by mu.
}

// func1 demonstrates not protected write access.
func (s *s1) func1() {
	s.protectedField1 = 42 // todo(mneverov): want this to fail
}

// func2 demonstrates protected write access.
func (s *s1) func2() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.protectedField2 = 42 // the access is protected, all is fine.
}

// func3 demonstrates not protected read access.
func (s *s1) func3() {
	_ = s.protectedField3    // todo(mneverov): want ...
	tmp := s.protectedField4 // todo(mneverov): want ...
	fmt.Println(tmp)
	fmt.Println(s.protectedField5)
	if s.protectedField6 > 0 { // todo(mneverov): ...
		if 42 > s.protectedField6 { // todo(mneverov): ...
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

// s3 is a struct with no protected fields.
type s3 struct {
	// field1
	field1 int
}

// weird comment outside struct that does not belong to anything
// but still declares that it is protected by something.
