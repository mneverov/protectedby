package protectedby

import "sync"

type embed struct {
	// protected by Mu. The mutex field Mu is intentionally public to make sure that it is not reported
	// in case of embedded fields.
	int
	Mu sync.Mutex
}
