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
	usagePositions  []token.Position
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
		return nil, nil
	}

	addUsages(pass, protectedMap)

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

func addUsages(pass *analysis.Pass, m map[string]*protected) {
	// in each file find all selector expressions
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
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
				// use named name to lookup a corresponding protected field.
				// todo(mneverov): is it possible to have an alias here? Then need to deAlias.
				pName := protectedName(xTypedName.Name(), se.Sel.Name)
				p, ok := m[pName]
				if !ok {
					// todo(mneverov): log with debug level that pName was not found. It can be either
					//  a regular field, or a missing "protected".
					return false
				}

				p.usagePositions = append(p.usagePositions, pass.Fset.Position(se.X.Pos()))
				return false
			}

			return true
		})
	}
}

// getLockName returns the first word in the comment after "protected by" statement or error if the statement is not
// found or found more than once.
func getLockName(comment *ast.Comment, testRun bool) (string, *analysisError) {
	text := comment.Text
	if testRun {
		if idx := strings.Index(comment.Text, testDirective); idx != -1 {
			text = text[:idx]
		}
	}

	lowerCaseComment := strings.ToLower(text)
	cnt := strings.Count(lowerCaseComment, protectedBy)
	if cnt != 1 {
		return "", &analysisError{
			msg: fmt.Sprintf("found %d %q in comment %q, expected exact one", cnt, protectedBy, text),
			pos: comment.Pos(),
		}
	}

	idx := strings.Index(lowerCaseComment, protectedBy)
	if idx == -1 {
		return "", &analysisError{
			msg: fmt.Sprintf("comment %q does not contain %q statement", text, protectedBy),
			pos: comment.Pos(),
		}
	}

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
