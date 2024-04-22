package protectedby

import "sync"

type embed struct {
	// protected by Mu. Make Mutex field public to make sure that it is not reported
	// in case of embedded field.
	int
	Mu sync.Mutex
}
