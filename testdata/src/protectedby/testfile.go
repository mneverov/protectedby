package protectedby

import (
	"fmt"
	"sync"
)

// This comment is associated with the hello constant.
const hello = "Hello, World!" // line comment 1

// This comment is associated with the foo variable.
var foo = hello // line comment 2

// main is a main function. It cannot be protected by anything.
func main() {
	fmt.Println(hello) // line comment 3
}

// s1 is a struct with some fields.
type s1 struct {
	// field1
	// protected by mu.
	field1 int
	field2 int // field2 protected by mu.
	/*
		field3 is
		protected by mu
		and also has a multiline comment.
	*/
	field3 int

	// field4 has a comment with empty line between the comment and field declaration.
	// protected by mu.

	field4 int

	// protected by mu.
	field5 int
	// protected by: mu.
	field6 int
	// protected by "mu".
	field7 int
	// field8 is protected by mu and protected by mu.// want "found 2 \"protected by \" in comment \"// field8 is protected by mu and protected by mu.\", expected exact one"
	field8 int

	// field10 is protected by not existing mutex.// want `struct "s1" does not have lock field "not"`
	field10 int
	mu      sync.Mutex

	// comment that does not belong to any field
	// but still protected by mu.
}

func (s *s1) func1() {
	s.field5 = 42
}

func (s *s1) func2() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.field5 = 42
}

// weird comment outside struct that does not belong to anything
// but still declares that it is protected by something.
