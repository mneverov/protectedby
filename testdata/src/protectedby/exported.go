package protectedby

import "sync"

type exportedProtected struct {
	// ProtectedField is protected by mu.
	ProtectedField int // want `exported protected field exportedProtected.ProtectedField`
	mu             sync.Mutex
}

type exportedMutex struct {
	// i is protected by ExportedMu.
	i          int
	ExportedMu sync.Mutex // want `exported mutex exportedMutex.ExportedMu`
}
