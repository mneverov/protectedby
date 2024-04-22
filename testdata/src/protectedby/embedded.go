package protectedby

import "sync"

type embed struct {
	// protected by mu.
	int
	mu sync.Mutex
}
