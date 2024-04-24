package protectedby

import "sync"

type nested struct {
	// i is protected by mu.
	i  int
	mu sync.Locker
}

type outer struct {
	n nested
}

func nestedAccess() {
	o := outer{}
	o.n.i = 42 //  todo(mneverov): want ...
}
