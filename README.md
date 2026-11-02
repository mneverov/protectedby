# protectedby

`protectedby` checks that access to struct fields is guarded by the lock their comment names.

```go
type someStruct struct {
	// i is protected by mu.
	i  int
	mu sync.Mutex
}

func foo() {
	s := someStruct{}
	s.i = 42 // not protected access to shared field i, use s.mu.Lock()
}

func bar() {
	s := someStruct{}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.i = 42 // ok
}
```

## Installation

```sh
go install github.com/mneverov/protectedby@latest
```

## Usage

Run it as a standalone tool:

```sh
protectedby ./...
```

Or as a `go vet` tool:

```sh
go vet -vettool=$(which protectedby) ./...
```

## Rules

- A comment on a struct field that contains `protected by <lock>` marks the field as protected. The match is
  case-insensitive, and the lock name is the first word after the directive, so `// Protected by mu.` works too.
- The lock must be a field of the same struct and implement `sync.Locker` (`Lock()` and `Unlock()`, on the type or its
  pointer), for example `sync.Mutex` or `sync.RWMutex`.
- The protected field and the lock must be unexported.
- Every access to a protected field, such as `s.i`, must come after `s.mu.Lock()` in the same function, with no
  `s.mu.Unlock()` in between. A deferred `Unlock()` is fine.
- `s.mu.RLock()` protects reads only. A write needs `s.mu.Lock()`. A write is an assignment (including `+=`), `++`,
  `--`, taking the address with `&`, `delete`, `clear` or `copy` into the field, a range loop that assigns to the
  field, or a pointer-receiver method call. Writes to elements and subfields count, such as `s.m[k] = v` or
  `s.c.v = 1`.

Every comment on a struct field that contains `protected by` is treated as a directive. A malformed one is reported,
for example `struct "S" does not have lock field "the"` for a comment like `// not protected by the mutex`. Only
that field is skipped; the rest of the package is still checked.

The [test data](./protectedby/testdata/src/protectedby) has an example for every case.

## Limitations

The checks are syntactic and local to a function, so some code is reported although it is correct (false positive),
and some incorrect code is not reported (false negative).

- **Control flow is not modeled.** "Before" means earlier in the source of the same function. An `Unlock()` in a
  branch that returns makes later accesses look unprotected, and a `Lock()` in a branch protects all later accesses
  ([`controlflow.go`](./protectedby/testdata/src/protectedby/controlflow.go)).
- **Closures belong to the function they are defined in.** A closure defined before `Lock()` and called after it is
  reported, and an `Unlock()` inside a called closure is not seen
  ([`nested.go`](./protectedby/testdata/src/protectedby/nested.go)).
- **A lock held by the caller is not seen.** A helper that expects its caller to hold the lock must lock it itself or
  its accesses are reported.
- **Only `x.field` is checked**, where `x` is a variable or parameter. A field reached through another field, such as
  `o.n.i`, is not checked ([`nested.go`](./protectedby/testdata/src/protectedby/nested.go)).
- **Promoted fields are not checked.** If `outer` embeds `inner`, the access `o.i` to the protected field `inner.i`
  is not checked.
- **Fields are identified by struct name and field name.** Two types with the same name in one package, such as types
  declared inside different functions, can be confused.
- **Struct literals are not checked**, for example `someStruct{i: 42}`.
- **Write detection for `RLock()` sees only the access itself.** A write through a copy (`m := s.m; m[k] = v`) or in a
  called function counts as a read.
- **A field declared together with others** (`a, b int // protected by mu`) is not checked.
