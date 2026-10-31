package protectedby

import "sync"

type aliasedStruct struct {
	// i is protected by mu.
	i  int
	mu sync.Mutex
}

type aliasStruct = aliasedStruct

type aliasOfAlias = aliasStruct

type aliasedGeneric[T any] struct {
	// v is protected by mu.
	v  T
	mu sync.Mutex
}

type aliasGeneric[T any] = aliasedGeneric[T]

func accessViaAlias() {
	var s aliasStruct
	s.i = 42 // want `not protected access to shared field i, use s.mu.Lock()`
}

func accessViaAliasPointer() {
	s := &aliasStruct{}
	s.i = 42 // want `not protected access to shared field i, use s.mu.Lock()`
}

func accessViaAliasOfAlias() {
	var s aliasOfAlias
	s.i = 42 // want `not protected access to shared field i, use s.mu.Lock()`
}

func accessViaGenericAlias() {
	var s aliasGeneric[int]
	s.v = 42 // want `not protected access to shared field v, use s.mu.Lock()`
}

func protectedAccessViaAlias() {
	var s aliasStruct
	s.mu.Lock()
	defer s.mu.Unlock()

	s.i = 42
}
