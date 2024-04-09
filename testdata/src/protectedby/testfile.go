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
	// multiline comment
	field1 int
	field2 int // field2 comment
	/*
		field 3 comment
		that is
		also a multiline.
	*/
	field3 int

	// field4 comment with empty line between the comment and field declaration.

	field4 int

	// protected by mu
	field5 int
	// protected by: mu
	field6 int
	// protected by "mu"
	field7 int
	// todo(mneverov): test multiple "protected by" statements protected_by lock1 protected_by lock2
	field8 int
	mu     sync.Mutex

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
