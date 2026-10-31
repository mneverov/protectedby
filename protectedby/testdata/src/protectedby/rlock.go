package protectedby

import "sync"

type counter struct{ v int }

func (c *counter) inc()    { c.v++ }
func (c counter) get() int { return c.v }

// embedsCounterPtr gets inc() and get() promoted from the embedded *counter.
type embedsCounterPtr struct{ *counter }

type rlockStruct struct {
	// n is protected by rw.
	n int
	// m is protected by rw.
	m map[string]int
	// sl is protected by rw.
	sl []int
	// c is protected by rw.
	c counter
	// p is protected by rw.
	p *counter
	// arr is protected by rw.
	arr [2]int
	// e is protected by rw.
	e  embedsCounterPtr
	rw sync.RWMutex
}

func readsUnderRLock() int {
	s := rlockStruct{}
	s.rw.RLock()
	defer s.rw.RUnlock()

	x := s.n + s.m["k"] + len(s.m) + s.sl[0] + s.c.v + s.c.get() + s.p.v + s.arr[0] + s.e.get()
	for k, v := range s.m {
		x += len(k) + v
	}
	return x
}

func writesUnderRLock() {
	s := rlockStruct{}
	s.rw.RLock()
	defer s.rw.RUnlock()

	s.n = 1              // want `not protected access to shared field n, use s.rw.Lock()`
	(s.n) = 2            // want `not protected access to shared field n, use s.rw.Lock()`
	s.n += 1             // want `not protected access to shared field n, use s.rw.Lock()`
	s.n++                // want `not protected access to shared field n, use s.rw.Lock()`
	_ = &s.n             // want `not protected access to shared field n, use s.rw.Lock()`
	s.m["k"] = 1         // want `not protected access to shared field m, use s.rw.Lock()`
	s.m["k"]++           // want `not protected access to shared field m, use s.rw.Lock()`
	delete(s.m, "k")     // want `not protected access to shared field m, use s.rw.Lock()`
	clear(s.m)           // want `not protected access to shared field m, use s.rw.Lock()`
	copy(s.sl, []int{1}) // want `not protected access to shared field sl, use s.rw.Lock()`
	s.sl[0] = 1          // want `not protected access to shared field sl, use s.rw.Lock()`
	s.c.v = 1            // want `not protected access to shared field c, use s.rw.Lock()`
	s.c.inc()            // want `not protected access to shared field c, use s.rw.Lock()`
	s.p.v = 1            // want `not protected access to shared field p, use s.rw.Lock()`
	*s.p = counter{}     // want `not protected access to shared field p, use s.rw.Lock()`
	for s.n = range 3 {  // want `not protected access to shared field n, use s.rw.Lock()`
	}
	for _, s.n = range []int{1} { // want `not protected access to shared field n, use s.rw.Lock()`
	}
	s.arr[0] = 1 // want `not protected access to shared field arr, use s.rw.Lock()`
	// inc() has a pointer receiver on the embedded *counter. It modifies the counter that e points to, not e itself,
	// but that data is still guarded by rw.
	s.e.inc() // want `not protected access to shared field e, use s.rw.Lock()`
}

func indexReadInWrite() {
	s := rlockStruct{}
	s.rw.RLock()
	defer s.rw.RUnlock()

	// Only sl is written. n is read as the index.
	s.sl[s.n] = 1 // want `not protected access to shared field sl, use s.rw.Lock()`
}

func writesUnderLock() {
	s := rlockStruct{}
	s.rw.Lock()
	defer s.rw.Unlock()

	s.n = 1
	s.m["k"] = 1
	s.c.inc()
}

func readAfterRUnlock() {
	s := rlockStruct{}
	s.rw.RLock()
	s.rw.RUnlock()

	_ = s.n // want `not protected access to shared field n, use s.rw.Lock()`
}

func writeUnderRLockAfterLock() {
	s := rlockStruct{}
	s.rw.Lock()
	s.rw.Unlock()
	s.rw.RLock()

	s.n = 1 // want `not protected access to shared field n, use s.rw.Lock()`
}

func readUnderRLockAfterLock() {
	s := rlockStruct{}
	s.rw.Lock()
	s.rw.Unlock()
	s.rw.RLock()

	_ = s.n
}
