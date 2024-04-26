# protectedby

Linter that checks concurrent access to shared resources.

```sh
go install github.com/mneverov/protectedby@latest
```

When a shared resource (field) is accompanied by the comment `protected by <lock_name>`, access to this field will be
validated to ensure it is guarded by the specified mutex.

See code snippet:

```go

type someStruct struct {
// i is protected by mu. 
i int
mu sync.Mutex
}

func foo() {
s := someStruct{}
s.i = 42 // not protected access to shared field i, use s.mu.Lock()
}
```

For more info see [tests](./testdata/src/protectedby).
