# protectedby

Checks that access to shared resources is protected.

```sh
go install github.com/mneverov/protectedby@latest
```

When a shared resource (field) has a comment with `protected by <lock_name>`, access to this field will be
validated to ensure it is guarded by the specified lock.

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

For more info see [tests](./protectedby/testdata/src/protectedby).
