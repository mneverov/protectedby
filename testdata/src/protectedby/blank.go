package protectedby

import "sync"

type blankIdent struct {
	// _ is protected by Mu. The mutex field Mu is intentionally public to make sure that it is not reported
	// in case of blank identifiers.
	_  int
	Mu sync.Mutex
}
