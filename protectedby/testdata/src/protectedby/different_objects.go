package protectedby

import "sync"

type difObjStruct struct {
	//i is protected by mu.
	i  int
	mu sync.Mutex
}

func differentObjects() {
	p1 := difObjStruct{}
	p2 := difObjStruct{}

	p1.mu.Lock()
	p2.i = 42 // want `not protected access to shared field i, use p2.mu.Lock()`
}
