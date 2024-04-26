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

type protected struct {
	field           *ast.Field
	lock            *ast.Field
	containerStruct *ast.TypeSpec
	structType      types.Type
	usagePositions  []*usagePosition
}

type usagePosition struct {
	expr         *ast.Expr
	file         *ast.File
	enclosingFun *ast.FuncDecl
}

var analyzer = &analysis.Analyzer{
	Name:     "protectedby",
	Doc:      "Checks concurrent access to shared resources.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	structs := getStructs(pass.Files, pass.Fset)
	protectedMap, errors := parseComments(pass, structs, testRun)
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
	return nil, nil
}

func parseComments(pass *analysis.Pass, fileStructs map[string]map[string]*ast.TypeSpec, testRun bool) (map[string]*protected, []*analysisError) {
	res := make(map[string]*protected)
	var errors []*analysisError
	for _, f := range pass.Files {
		fileName := pass.Fset.Position(f.Pos()).Filename
		commentMap := ast.NewCommentMap(pass.Fset, f, f.Comments)

		for node, commentMapGroups := range commentMap {
			// Filter out nodes that are not fields. The linter only works for a struct fields protected
			// by another field.
			field, ok := node.(*ast.Field)
			if !ok {
				continue
			}
			// skip embedded fields and blank identifiers
			if len(field.Names) != 1 || field.Names[0].Name == "_" {
				continue
			}

			fieldName := field.Names[0].Name
			for _, commentGroup := range commentMapGroups {
				// todo(mneverov): when a comment is a multiline comment
				//  and on each line "protected by " is mentioned, then it should be handled differently.
				//  maybe add an error when another "protected by" is found, or when find the first "protected by"
				//  then break (current behavior).

				// In each case when get an error -- return early, i.e. break out from the loop.
				for _, c := range commentGroup.List {
					if !strings.Contains(strings.ToLower(c.Text), protectedBy) {
						continue
					}

					spec, err := getStructSpec(field, fileStructs[fileName])
					if err != nil {
						errors = append(errors, err)
						break
					}

					// check here instead out of the loop because want to skip struct search for fields without
					// "protected by".
					if token.IsExported(fieldName) {
						errors = append(errors, &analysisError{
							msg: fmt.Sprintf("exported protected field %s.%s", spec.Name, fieldName),
							pos: field.Pos(),
						})
						break
					}
					// todo(mneverov): use pass.pkg.scope.parent.lookup instead
					lock, err := getLockField(c, spec, testRun)
					if err != nil {
						errors = append(errors, err)
						break
					}

					lockName := lock.Names[0].Name
					if !implementsLocker(pass, lock) {
						errors = append(errors, &analysisError{
							msg: fmt.Sprintf("lock %s doesn't implement sync.Locker interface", lockName),
							pos: lock.Pos(),
						})
						break
					}
					if token.IsExported(lock.Names[0].Name) {
						errors = append(errors, &analysisError{
							msg: fmt.Sprintf("exported mutex %s.%s", spec.Name, lockName),
							pos: lock.Pos(),
						})
						break
					}

					p := &protected{
						field:           field,
						lock:            lock,
						containerStruct: spec,
						structType:      pass.TypesInfo.TypeOf(spec.Type),
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

func getLockField(comment *ast.Comment, ts *ast.TypeSpec, testRun bool) (*ast.Field, *analysisError) {
	lockName, err := getLockName(comment, testRun)
	if err != nil {
		return nil, err
	}

	st := ts.Type.(*ast.StructType)
	if st.Fields == nil {
		// Presence of the struct Name is checked in getStructSpec.
		return nil, &analysisError{
			msg: fmt.Sprintf("failed to find lock %q in struct %s: fieldList is nil", lockName, ts.Name.Name),
			pos: comment.Pos(),
		}
	}

	var lockField *ast.Field
	found := false
	for _, field := range st.Fields.List {
		for _, n := range field.Names {
			if lockName == n.Name {
				lockField = field
				found = true
				break
			}
		}
	}

	if !found {
		return nil, &analysisError{
			msg: fmt.Sprintf("struct %q does not have lock field %q", ts.Name.Name, lockName),
			pos: comment.Pos(),
		}
	}

	return lockField, nil
}

func implementsLocker(pass *analysis.Pass, f *ast.Field) bool {
	realType := pass.TypesInfo.TypeOf(f.Type)
	ptrType := types.NewPointer(realType)
	return types.Implements(realType, syncLocker) || types.Implements(ptrType, syncLocker)
}

func getStructSpec(field ast.Node, structs map[string]*ast.TypeSpec) (*ast.TypeSpec, *analysisError) {
	found := false
	var fieldStruct *ast.TypeSpec
	for _, s := range structs {
		if s.Pos() <= field.Pos() && s.End() >= field.End() {
			fieldStruct = s
			found = true
			break
		}
	}

	if !found {
		return nil, &analysisError{
			msg: fmt.Sprintf("failed to find a struct for %s", field),
			pos: field.Pos(),
		}
	}

	return fieldStruct, nil
}

func getStructs(files []*ast.File, fset *token.FileSet) map[string]map[string]*ast.TypeSpec {
	structs := make(map[string]map[string]*ast.TypeSpec)
	for _, f := range files {
		fileName := fset.Position(f.Pos()).Filename
		fileStructs := getFileStructs(f.Decls)
		structs[fileName] = fileStructs
	}

	return structs
}

func getFileStructs(decls []ast.Decl) map[string]*ast.TypeSpec {
	structs := make(map[string]*ast.TypeSpec)
	for _, decl := range decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if typeSpec.Name == nil {
						continue
					}
					if _, ok := typeSpec.Type.(*ast.StructType); ok {
						structs[typeSpec.Name.Name] = typeSpec
					}
				}
			}
		}
	}

	return structs
}

// todo(mneverov): better naming func and map
func addUsages(pass *analysis.Pass, m map[string]*protected) []*analysisError {
	var errors []*analysisError
	// in each file find all selector expressions
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch se := n.(type) {
			case *ast.SelectorExpr:
				if _, ok := se.X.(*ast.Ident); !ok {
					return false
				}

				xtyp := deref(pass.TypesInfo.TypeOf(se.X))
				// the type of the accessor must be named
				named, ok := xtyp.(*types.Named)
				if !ok {
					return false
				}
				xTypedName := named.Obj()
				// use typed name to lookup a corresponding protected field.
				// todo(mneverov): is it possible to have an alias here? Then need to deAlias.
				pName := protectedName(xTypedName.Name(), se.Sel.Name)
				p, ok := m[pName]
				if !ok {
					return false
				}

				fun, err := findEnclosingFunction(se.X.Pos(), se.X.End(), file)
				if err != nil {
					errors = append(errors, err)
					return false
				}

				p.usagePositions = append(p.usagePositions, &usagePosition{
					file:         file,
					expr:         &se.X,
					enclosingFun: fun,
				})
				return false
			}

			return true
		})
	}

	return errors
}

func findEnclosingFunction(start, end token.Pos, file *ast.File) (*ast.FuncDecl, *analysisError) {
	path, _ := astutil.PathEnclosingInterval(file, start, end)

	// Find the function declaration that encloses the positions.
	var outer *ast.FuncDecl
	for _, p := range path {
		/*
			todo(mneverov): check defer. Currently, this does not recognize deferred functions, so for the following
			func notProtectedAccessInDefer() { <--- current enclosing
				s := s1{}
				defer func() {                 <--- should be this instead
					s.protectedField1 = 42 // todo(mneverov): want ...
				}()
			}
			the enclosed function is "notProtectedAccessInDefer" but should be deferred function
		*/
		if p, ok := p.(*ast.FuncDecl); ok {
			outer = p
			break
		}
	}
	if outer == nil {
		// todo(mneverov): add test example when a struct field is a package variable.
		return nil, &analysisError{
			msg: "no enclosing function",
			pos: start,
		}

	}

	return outer, nil
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
