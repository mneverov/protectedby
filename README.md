# protectedby
Linter that checks concurrent access to shared resources

See code snippet:

```go

type s struct {
	// field1 is protected by mu. 
	field1 int
	
	// field2 is protected by mu. Only first encounter of "protected by " is checked
	// field2 is also protected by something else. This line is ignored!
	field2 int
	
	mu sync.Mutex
}
```
