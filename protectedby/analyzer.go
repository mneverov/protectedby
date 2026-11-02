package protectedby

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/edge"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	protectedBy   = "protected by "
	testDirective = "// want `"
)

var testRun bool

var syncLocker = types.NewInterfaceType(
	[]*types.Func{
		types.NewFunc(token.NoPos, nil, "Lock",
			types.NewSignatureType(nil, nil, nil, types.NewTuple(), types.NewTuple(), false)),
		types.NewFunc(token.NoPos, nil, "Unlock",
			types.NewSignatureType(nil, nil, nil, types.NewTuple(), types.NewTuple(), false)),
	},
	nil,
).Complete()

type analysisError struct {
	msg string
	pos token.Pos
}

func (e analysisError) Error() string {
	return e.msg
}

type protectedData struct {
	field *ast.Field
	// lockName is the lock field's name from the comment. A field such as "muA, muB sync.Mutex" has several names,
	// so the name cannot be taken from the lock's *ast.Field.
	lockName        string
	enclosingStruct *ast.TypeSpec
	usages          []*usage
}

type usage struct {
	selectorXID *ast.Ident
	// enclosingFunc points to the *ast.FuncDecl that contains the access.
	enclosingFunc inspector.Cursor
	deferStmt     *ast.DeferStmt
	// write is true if the access may modify the field, see isWrite. For reads, RLock() is enough.
	write bool
}

var Analyzer = &analysis.Analyzer{
	Name:     "protectedby",
	Doc:      "Checks that access to shared resources is protected.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

// run reports the errors of all phases. A malformed comment only drops its own field, so the remaining fields are
// still checked.
func run(pass *analysis.Pass) (any, error) {
	root := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector).Root()
	protectedMap, errors := parseComments(pass, root)
	errors = append(errors, addUsages(pass, root, protectedMap)...)
	errors = append(errors, checkLocksUsed(pass, protectedMap)...)
	for _, e := range errors {
		pass.Report(analysis.Diagnostic{Pos: e.pos, Message: e.Error()})
	}
	return nil, nil
}

func parseComments(pass *analysis.Pass, root inspector.Cursor) (map[string]*protectedData, []*analysisError) {
	res := make(map[string]*protectedData)
	var errors []*analysisError

	for _, f := range pass.Files {
		commentMap := ast.NewCommentMap(pass.Fset, f, f.Comments)

		for node, commentMapGroups := range commentMap {
			// Filter out nodes that are not fields. The linter only works for struct fields protected
			// by another field.
			field, ok := node.(*ast.Field)
			if !ok {
				continue
			}
			// Skip embedded fields and blank identifiers.
			fieldName := getFieldName(field)
			if fieldName == "" || fieldName == "_" {
				continue
			}

		commentGroup:
			for _, cg := range commentMapGroups {
				for _, comment := range cg.List {
					if !strings.Contains(strings.ToLower(comment.Text), protectedBy) {
						continue
					}

					spec := getEnclosingStruct(root, comment.Pos(), comment.End())
					if spec == nil {
						continue commentGroup
					}

					if token.IsExported(fieldName) {
						errors = append(errors, &analysisError{
							msg: fmt.Sprintf("exported protected field %s.%s", spec.Name, fieldName),
							pos: field.Pos(),
						})
						continue commentGroup
					}

					lock, lockName, err := getLock(pass, spec, comment)
					if err != nil {
						errors = append(errors, err)
						continue commentGroup
					}

					// A comment about the fields guarded by a lock is often attached to the lock itself, e.g.
					// "// lines and infos are protected by mutex" above the mutex field in go/token.File.
					// A lock cannot protect itself.
					if lock == field {
						continue commentGroup
					}

					p := &protectedData{
						field:           field,
						lockName:        lockName,
						enclosingStruct: spec,
					}

					pName := protectedName(spec.Name.Name, fieldName)
					res[pName] = p
					break
				}
			}
		}
	}

	return res, errors
}

func implementsLocker(pass *analysis.Pass, f *ast.Field) bool {
	realType := pass.TypesInfo.TypeOf(f.Type)
	ptrType := types.NewPointer(realType)
	return types.Implements(realType, syncLocker) || types.Implements(ptrType, syncLocker)
}

func addUsages(pass *analysis.Pass, root inspector.Cursor, protectedMap map[string]*protectedData) []*analysisError {
	var errors []*analysisError

	for cur := range root.Preorder((*ast.SelectorExpr)(nil)) {
		se := cur.Node().(*ast.SelectorExpr)
		id, ok := se.X.(*ast.Ident)
		if !ok {
			// In s.f.x and s.f.m() the protected field is the inner selector s.f, which Preorder visits next.
			continue
		}

		xtyp := deref(pass.TypesInfo.TypeOf(se.X))
		// The type of the accessor must be named.
		named, ok := xtyp.(*types.Named)
		if !ok {
			continue
		}

		// Use typed name to lookup a corresponding protected field.
		pName := protectedName(named.Obj().Name(), se.Sel.Name)
		p, ok := protectedMap[pName]
		if !ok {
			continue
		}

		fn, deferStmt, ok := enclosingFunction(cur)
		if !ok {
			errors = append(errors, &analysisError{
				msg: "no enclosing function",
				pos: se.X.Pos(),
			})
			continue
		}

		p.usages = append(p.usages, &usage{
			selectorXID:   id,
			enclosingFunc: fn,
			deferStmt:     deferStmt,
			write:         isWrite(pass.TypesInfo, cur),
		})
	}

	return errors
}

// isWrite reports whether the access to the field selected by cur may modify the field or the data it refers to.
// Starting from cur, it walks up through expressions that project the field (s.f.x, s.f[i], *s.f, s.f[i:j], (s.f))
// and classifies the edge to the first other ancestor. Writes through copies (m := s.f; m[k] = v) or through callees
// are not detected.
func isWrite(info *types.Info, cur inspector.Cursor) bool {
	for ; ; cur = cur.Parent() {
		k, idx := cur.ParentEdge()
		switch k {
		case edge.ParenExpr_X, edge.StarExpr_X, edge.IndexExpr_X, edge.SliceExpr_X:
		case edge.SelectorExpr_X:
			// A method call on the field modifies it if the receiver is a pointer: s.f.inc().
			if sel, ok := info.Selections[cur.Parent().Node().(*ast.SelectorExpr)]; ok && sel.Kind() == types.MethodVal {
				_, ptrRecv := sel.Obj().(*types.Func).Signature().Recv().Type().(*types.Pointer)
				return ptrRecv
			}
		case edge.AssignStmt_Lhs, edge.IncDecStmt_X:
			return true
		case edge.UnaryExpr_X:
			return cur.Parent().Node().(*ast.UnaryExpr).Op == token.AND
		case edge.RangeStmt_Key, edge.RangeStmt_Value:
			// "for s.f = range x" and "for _, s.f = range x" assign the field on every iteration. With ":=" the key
			// and value are new variables, so a field cannot appear there. Ranging over the field, as in
			// "for k := range s.f", is a read: edge.RangeStmt_X.
			return cur.Parent().Node().(*ast.RangeStmt).Tok == token.ASSIGN
		case edge.CallExpr_Args:
			// delete(s.m, k), clear(s.m), copy(s.sl, src).
			return idx == 0 && isBuiltin(info, cur.Parent().Node().(*ast.CallExpr).Fun, "delete", "clear", "copy")
		default:
			// Includes the field used as an index or a slice bound (s.sl[s.n], s.sl[s.n:]), the right-hand side of
			// an assignment, and the top of the tree (edge.Invalid).
			return false
		}
	}
}

func isBuiltin(info *types.Info, fun ast.Expr, names ...string) bool {
	id, ok := ast.Unparen(fun).(*ast.Ident)
	if !ok {
		return false
	}
	b, ok := info.Uses[id].(*types.Builtin)
	return ok && slices.Contains(names, b.Name())
}

// enclosingFunction returns the FuncDecl that contains cur and the outermost DeferStmt between them, if any. Deferred
// access to a protected field is validated differently.
func enclosingFunction(cur inspector.Cursor) (inspector.Cursor, *ast.DeferStmt, bool) {
	var deferStmt *ast.DeferStmt
	for c := range cur.Enclosing((*ast.FuncDecl)(nil), (*ast.DeferStmt)(nil)) {
		switch n := c.Node().(type) {
		case *ast.DeferStmt:
			deferStmt = n
		case *ast.FuncDecl:
			return c, deferStmt, true
		}
	}
	return inspector.Cursor{}, nil, false
}

func checkLocksUsed(pass *analysis.Pass, m map[string]*protectedData) []*analysisError {
	var errors []*analysisError
	for _, p := range m {
		for _, u := range p.usages {
			found := false
			var lockExpr *ast.SelectorExpr
			body := u.enclosingFunc.Node().(*ast.FuncDecl).Body
			u.enclosingFunc.Inspect([]ast.Node{(*ast.DeferStmt)(nil), (*ast.CallExpr)(nil)}, func(c inspector.Cursor) bool {
				curr := c.Node()

				// Access to a protected field can be deferred. Skip node if this is a defer statement
				// that is not the same where the field is accessed.
				if deferStmt, ok := curr.(*ast.DeferStmt); ok {
					return deferStmt == u.deferStmt
				}

				fnSelector, ok := curr.(*ast.CallExpr).Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}

				// Return if the function is not "Lock", or "RLock" for a read. At this point we already checked that
				// the lock implements sync.Locker interface, namely Lock() function, hence it cannot have another
				// function with a name Lock and arguments -- overloading is forbidden in go. RLock() compiles only
				// if the lock has it, e.g. sync.RWMutex.
				name := fnSelector.Sel.Name
				if name != "Lock" && (u.write || name != "RLock") {
					return true
				}

				// Return if the function call is outside the function or after the protected field access.
				if curr.Pos() <= body.Pos() || curr.Pos() >= u.selectorXID.Pos() {
					return false
				}

				if !isLockFieldOf(pass, fnSelector, p.lockName, u.selectorXID) {
					return true
				}

				// Inspect continues with the siblings, so the last Lock() before the access wins and
				// Lock(); Unlock(); Lock() counts as locked.
				found = true
				lockExpr = fnSelector
				return false
			})

			if !found || isUnlockCalled(pass, u, lockExpr, p.lockName) {
				errors = append(errors, &analysisError{
					msg: fmt.Sprintf("not protected access to shared field %s, use %s.%s.Lock()",
						getFieldName(p.field),
						u.selectorXID.Name,
						p.lockName,
					),
					pos: u.selectorXID.Pos(),
				})
			}
		}
	}

	return errors
}

func isUnlockCalled(pass *analysis.Pass, u *usage, lockExpr *ast.SelectorExpr, lockName string) bool {
	unlocked := false
	u.enclosingFunc.Inspect([]ast.Node{(*ast.CallExpr)(nil)}, func(c inspector.Cursor) bool {
		curr := c.Node()
		fnSelector, ok := curr.(*ast.CallExpr).Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		if fnSelector.Sel.Name != "Unlock" && fnSelector.Sel.Name != "RUnlock" {
			return true
		}

		withinRange := curr.Pos() > lockExpr.Pos() && curr.Pos() < u.selectorXID.Pos()
		// Return if the Unlock() is called before Lock() or after access to the protected field.
		if !withinRange {
			return false
		}

		if !isLockFieldOf(pass, fnSelector, lockName, u.selectorXID) {
			return true
		}

		// An Unlock() in another deferred statement does not run before the access, so it neither
		// unlocks nor cancels an earlier Unlock().
		_, deferStmt, _ := enclosingFunction(c)
		if deferStmt == u.deferStmt {
			unlocked = true
		}
		return false
	})

	return unlocked
}

// isLockFieldOf reports whether fnSelector is <x>.<lockName>.<Method> where x is the same object as accessor.
func isLockFieldOf(pass *analysis.Pass, fnSelector *ast.SelectorExpr, lockName string, accessor *ast.Ident) bool {
	/*
		Function expression must be a SelectorExpr because the following is not valid:
		s := s1{}      // a struct with a protected field and a mutex mu.
		copyMu := s.mu // this copies mutex, i.e. copyMu.Lock() will not protect the field. This is reported
			           // by go vet: "assignment copies lock value to mu: sync.Mutex".
		copyMu.Lock()
	*/
	selExpr, ok := fnSelector.X.(*ast.SelectorExpr)
	if !ok || selExpr.Sel.Name != lockName {
		return false
	}

	sxid, ok := selExpr.X.(*ast.Ident)
	if !ok {
		return false
	}

	return pass.TypesInfo.ObjectOf(sxid) == pass.TypesInfo.ObjectOf(accessor)
}

// getEnclosingStruct returns the innermost named struct type declaration that contains the range, e.g. a comment.
func getEnclosingStruct(root inspector.Cursor, posStart, posEnd token.Pos) *ast.TypeSpec {
	cur, ok := root.FindByPos(posStart, posEnd)
	if !ok {
		return nil
	}
	// Need TypeSpec here to get the struct name.
	for c := range cur.Enclosing((*ast.TypeSpec)(nil)) {
		typeSpec := c.Node().(*ast.TypeSpec)
		if _, ok := typeSpec.Type.(*ast.StructType); ok && typeSpec.Name != nil {
			return typeSpec
		}
	}
	return nil
}

func getLock(pass *analysis.Pass, spec *ast.TypeSpec, c *ast.Comment) (*ast.Field, string, *analysisError) {
	lockName, err := getLockName(c, testRun)
	if err != nil {
		return nil, "", err
	}

	lock := getStructFieldByName(lockName, spec.Type.(*ast.StructType))
	if lock == nil {
		return nil, "", &analysisError{
			msg: fmt.Sprintf("struct %q does not have lock field %q", spec.Name.Name, lockName),
			pos: c.Pos(),
		}
	}

	// Check if the lock field is exported after verifying that it exists. Otherwise may report
	// "exported mutex" for not existing field.
	if token.IsExported(lockName) {
		return nil, "", &analysisError{
			msg: fmt.Sprintf("exported mutex %s.%s", spec.Name.Name, lockName),
			pos: lock.Pos(),
		}
	}

	if !implementsLocker(pass, lock) {
		return nil, "", &analysisError{
			msg: fmt.Sprintf("lock %s doesn't implement sync.Locker interface", lockName),
			pos: lock.Pos(),
		}
	}

	return lock, lockName, nil
}

func getStructFieldByName(name string, st *ast.StructType) *ast.Field {
	for _, field := range st.Fields.List {
		for _, n := range field.Names {
			if name == n.Name {
				return field
			}
		}
	}

	return nil
}

// getLockName returns the first word in the comment after "protected by" statement or error if the statement is not
// found or found more than once.
func getLockName(comment *ast.Comment, testRun bool) (string, *analysisError) {
	text := comment.Text
	// analysistest uses comments of the form "// want ..." as an expected error message. A comment in a test file looks
	// like "is protected by not existing mutex.// want `struct "s1" does not have lock field "not"`" i.e. contains
	// multiple "protected by"'s. Since the analyser reacts on each "protected by" the code below excludes test
	// directives from "// want `" till the end of the comment line.
	if testRun {
		if idx := strings.Index(comment.Text, testDirective); idx != -1 {
			text = text[:idx]
		}
	}

	// Compare "protected by " directive with lowercase comment because the directive can be a separate sentence i.e.
	// starts with capital letter.
	lowerCaseComment := strings.ToLower(text)
	cnt := strings.Count(lowerCaseComment, protectedBy)
	if cnt != 1 {
		return "", &analysisError{
			msg: fmt.Sprintf("found %d %q in comment %q, expected exact one", cnt, protectedBy, text),
			pos: comment.Pos(),
		}
	}

	// The index of "protected by " directive is guaranteed to be greater than -1 by checking count above.
	idx := strings.Index(lowerCaseComment, protectedBy)
	c := text[idx+len(protectedBy):]
	fields := strings.FieldsFunc(c, isLetterOrNumber)
	if len(fields) == 0 {
		return "", &analysisError{
			msg: fmt.Sprintf("failed to parse lock name from comment %q", text),
			pos: comment.Pos(),
		}
	}

	return fields[0], nil
}

func isLetterOrNumber(c rune) bool {
	return !unicode.IsLetter(c) && !unicode.IsNumber(c)
}

// deref strips aliases and pointers, e.g. *A with type A = S yields S.
func deref(T types.Type) types.Type {
	T = types.Unalias(T)
	if p, ok := T.Underlying().(*types.Pointer); ok {
		return deref(p.Elem())
	}
	return T
}

func protectedName(structName, fieldName string) string {
	return fmt.Sprintf("%s.%s", structName, fieldName)
}

func getFieldName(f *ast.Field) string {
	if len(f.Names) != 1 {
		return ""
	}
	return f.Names[0].Name
}
