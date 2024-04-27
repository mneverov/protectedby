package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/astutil"
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
	field           *ast.Field
	lock            *ast.Field
	enclosingStruct *ast.TypeSpec
	usages          []*usage
}

type usage struct {
	file          *ast.File
	selectorXID   *ast.Ident
	enclosingFunc *ast.FuncDecl
	deferStmt     *ast.DeferStmt
}

var analyzer = &analysis.Analyzer{
	Name:     "protectedby",
	Doc:      "Checks that access to shared resources is protected.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	protectedMap, errors := parseComments(pass)
	if errors != nil {
		for _, e := range errors {
			pass.Reportf(e.pos, e.Error())
		}
		if !testRun {
			return nil, nil
		}
	}

	errors = addUsages(pass, protectedMap)
	if errors != nil {
		for _, e := range errors {
			pass.Reportf(e.pos, e.Error())
		}
		if !testRun {
			return nil, nil
		}
	}

	errors = checkLocksUsed(pass, protectedMap)
	if errors != nil {
		for _, e := range errors {
			pass.Reportf(e.pos, e.Error())
		}
		if !testRun {
			return nil, nil
		}
	}

	return nil, nil
}

func parseComments(pass *analysis.Pass) (map[string]*protectedData, []*analysisError) {
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

					spec := getEnclosingStruct(f, comment.Pos(), comment.End())
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

					lock, err := getLock(pass, spec, comment)
					if err != nil {
						errors = append(errors, err)
						continue commentGroup
					}

					p := &protectedData{
						field:           field,
						lock:            lock,
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

func addUsages(pass *analysis.Pass, protectedMap map[string]*protectedData) []*analysisError {
	var errors []*analysisError

	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch se := n.(type) {
			case *ast.SelectorExpr:
				id, ok := se.X.(*ast.Ident)
				if !ok {
					return false
				}

				xtyp := deref(pass.TypesInfo.TypeOf(se.X))
				// The type of the accessor must be named.
				named, ok := xtyp.(*types.Named)
				if !ok {
					return false
				}

				// Use typed name to lookup a corresponding protected field.
				pName := protectedName(named.Obj().Name(), se.Sel.Name)
				p, ok := protectedMap[pName]
				if !ok {
					return false
				}

				fn, deferStmt, err := findEnclosingFunction(se.X.Pos(), se.X.End(), file)
				if err != nil {
					errors = append(errors, err)
					return false
				}

				p.usages = append(p.usages, &usage{
					file:          file,
					selectorXID:   id,
					enclosingFunc: fn,
					deferStmt:     deferStmt,
				})

				return false
			}

			return true
		})
	}

	return errors
}

func findEnclosingFunction(start, end token.Pos, file *ast.File) (*ast.FuncDecl, *ast.DeferStmt, *analysisError) {
	path, _ := astutil.PathEnclosingInterval(file, start, end)

	// Find the function declaration that encloses the positions.
	var outer *ast.FuncDecl
	var deferStmt *ast.DeferStmt
	for _, p := range path {
		// Deferred access to a protected field is validated differently.
		if d, ok := p.(*ast.DeferStmt); ok {
			deferStmt = d
		}

		if p, ok := p.(*ast.FuncDecl); ok {
			outer = p
			break
		}
	}

	if outer == nil {
		return nil, nil, &analysisError{
			msg: "no enclosing function",
			pos: start,
		}
	}

	return outer, deferStmt, nil
}

func checkLocksUsed(pass *analysis.Pass, m map[string]*protectedData) []*analysisError {
	var errors []*analysisError
	for _, p := range m {
		for _, u := range p.usages {
			found := false
			var lockExpr *ast.SelectorExpr
			ast.Inspect(u.enclosingFunc, func(curr ast.Node) bool {
				if curr == nil {
					return false
				}

				// Access to a protected field can be deferred. Skip node if this is a defer statement
				// that is not the same where the field is accessed.
				if deferStmt, ok := curr.(*ast.DeferStmt); ok {
					if deferStmt == u.deferStmt {
						return true
					} else {
						return false
					}
				}

				cexpr, ok := curr.(*ast.CallExpr)
				if !ok {
					return true
				}
				fnSelector, ok := cexpr.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}

				// Return if the function is not "Lock". At this point we already checked that the lock implements
				// sync.Locker interface, namely Lock() function, hence it cannot have another function with
				// a name Lock and arguments -- overloading is forbidden in go.
				if fnSelector.Sel.Name != "Lock" {
					return true
				}

				// Return if the function call is outside the function or after the protected field access.
				if curr.Pos() <= u.enclosingFunc.Body.Pos() || curr.Pos() >= u.selectorXID.Pos() {
					return false
				}

				/*
					Function expression must be a SelectorExpr because the following is not valid:
					s := s1{}      // a struct with a protected field and a mutex mu.
					copyMu := s.mu // this copies mutex, i.e. copyMu.Lock() will not protect the field. This is reported
						           // by go vet: "assignment copies lock value to mu: sync.Mutex".
					copyMu.Lock()
				*/
				selExpr, ok := fnSelector.X.(*ast.SelectorExpr)
				if !ok {
					return true
				}

				sxid, ok := selExpr.X.(*ast.Ident)
				if !ok {
					return true
				}

				if pass.TypesInfo.ObjectOf(sxid) == pass.TypesInfo.ObjectOf(u.selectorXID) {
					found = true
					lockExpr = fnSelector
					return false
				}

				return true
			})

			if !found || isUnlockCalled(pass, u, lockExpr) {
				errors = append(errors, &analysisError{
					msg: fmt.Sprintf("not protected access to shared field %s, use %s.%s.Lock()",
						getFieldName(p.field),
						u.selectorXID.Name,
						getFieldName(p.lock),
					),
					pos: u.selectorXID.Pos(),
				})
			}
		}
	}

	return errors
}

func isUnlockCalled(pass *analysis.Pass, u *usage, lockExpr *ast.SelectorExpr) bool {
	unlocked := false
	ast.Inspect(u.enclosingFunc, func(curr ast.Node) bool {
		if curr == nil {
			return false
		}

		cexpr, ok := curr.(*ast.CallExpr)
		if !ok {
			return true
		}
		fnSelector, ok := cexpr.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		if fnSelector.Sel.Name != "Unlock" {
			return true
		}

		withinRange := curr.Pos() > lockExpr.Pos() && curr.Pos() < u.selectorXID.Pos()
		// Return if the Unlock() is called before Lock() or after access to the protected field.
		if !withinRange {
			return false
		}

		/*
			function expression must be a SelectorExpr because the following is not valid:
			s := s1{}      // a struct with a protected field and a mutex mu.
			copyMu := s.mu // this copies mutex, i.e. copyMu.Lock() will not protect the field. This is reported
				           // by go vet: "assignment copies lock value to mu: sync.Mutex".
			copyMu.Lock()
		*/
		selExpr, ok := fnSelector.X.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		sxid, ok := selExpr.X.(*ast.Ident)
		if !ok {
			return true
		}

		if pass.TypesInfo.ObjectOf(sxid) == pass.TypesInfo.ObjectOf(u.selectorXID) {
			// If Unlock() is called from within deferred function
			_, deferStmt, _ := findEnclosingFunction(curr.Pos(), curr.End(), u.file)
			// it must be the same deferred statement as the deferred statement where current usage happened.
			unlocked = deferStmt == u.deferStmt
			return false
		}

		return true
	})

	return unlocked
}

func getEnclosingStruct(f *ast.File, posStart, posEnd token.Pos) *ast.TypeSpec {
	// Need TypeSpec here to get the struct name.
	var spec *ast.TypeSpec
	path, _ := astutil.PathEnclosingInterval(f, posStart, posEnd)
	for _, p := range path {
		if typeSpec, ok := p.(*ast.TypeSpec); ok {
			if typeSpec.Name == nil {
				continue
			}
			if _, ok := typeSpec.Type.(*ast.StructType); ok {
				spec = typeSpec
				break
			}
		}
	}

	return spec
}

func getLock(pass *analysis.Pass, spec *ast.TypeSpec, c *ast.Comment) (*ast.Field, *analysisError) {
	lockName, err := getLockName(c, testRun)
	if err != nil {
		return nil, err
	}

	lock := getStructFieldByName(lockName, spec.Type.(*ast.StructType))
	if lock == nil {
		return nil, &analysisError{
			msg: fmt.Sprintf("struct %q does not have lock field %q", spec.Name.Name, lockName),
			pos: c.Pos(),
		}
	}

	// Check if the lock field is exported after verifying that it exists. Otherwise may report
	// "exported mutex" for not existing field.
	if token.IsExported(lockName) {
		return nil, &analysisError{
			msg: fmt.Sprintf("exported mutex %s.%s", spec.Name.Name, lockName),
			pos: lock.Pos(),
		}
	}

	if !implementsLocker(pass, lock) {
		return nil, &analysisError{
			msg: fmt.Sprintf("lock %s doesn't implement sync.Locker interface", lockName),
			pos: lock.Pos(),
		}
	}

	return lock, nil
}

func getStructFieldByName(name string, st *ast.StructType) *ast.Field {
	var lockField *ast.Field

	for _, field := range st.Fields.List {
		for _, n := range field.Names {
			if name == n.Name {
				lockField = field
				break
			}
		}
	}

	return lockField
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

func deref(T types.Type) types.Type {
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
