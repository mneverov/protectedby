package protectedby

import "sync"

type blankIdent struct {
	// _ is protected by mu.
	_  int
	mu sync.Mutex
}
