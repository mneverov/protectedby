package malformed

import "sync"

// A malformed comment is reported and skipped. The other fields of the package are still checked.
type config struct {
	// keys is not protected by the mutex.// want `struct "config" does not have lock field "the"`
	keys []string
	// n is protected by mu.
	n  int
	mu sync.Mutex
}

func access() {
	c := config{}
	_ = c.keys
	c.n = 1 // want `not protected access to shared field n, use c.mu.Lock()`
}
