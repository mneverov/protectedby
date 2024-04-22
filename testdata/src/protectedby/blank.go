package protectedby

import "sync"

type blankIdent struct {
	// _ is protected by Mu. Make Mutex field public to make sure that it is not reported
	// in case of embedded field.
	_  int
	Mu sync.Mutex
}
