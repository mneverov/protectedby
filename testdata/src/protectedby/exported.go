package protectedby

import "sync"

type s8 struct {
	// S8f2 is protected by s8mu.
	S8f2 int // want `exported protected field s8.S8f2`
	s8mu sync.Mutex
}

type s9 struct {
	// s9f1 is protected by S9mu.
	s9f1 int
	S9mu sync.Mutex // want `exported mutex s9.S9mu`
}
