package protectedby

import "sync"

type exportedProtectedStruct struct {
	// ProtectedField is protected by mu.
	ProtectedField int // want `exported protected field exportedProtectedStruct.ProtectedField`
	mu             sync.Mutex
}

type exportedMutexStruct struct {
	// i is protected by ExportedMu.
	i          int
	ExportedMu sync.Mutex // want `exported mutex exportedMutexStruct.ExportedMu`
}
