package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"log"
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

func init() {
	analyzer.Flags.BoolVar(&testRun, "testrun", false, "if true, comments started with // want ` will be ignored")
}

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
	file            *ast.File
	fset            *token.FileSet
}

var analyzer = &analysis.Analyzer{
	Name:     "protectedby",
	Doc:      "Checks concurrent access to shared resources.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	// todo(mneverov): use package instead of full path?
	log.Printf("package %q", pass.Pkg.Name())
	structsPerFile := getStructs(pass.Files, pass.Fset)
	fps, errors := parseComments(pass.Files, pass.Fset, structsPerFile, testRun)
	if errors != nil {
		for _, e := range errors {
			pass.Reportf(e.pos, e.Error())
		}
	}

	println(protectedToLocks)

	return nil, nil
}

func parseComments(files []*ast.File, fset *token.FileSet, fileStructs map[string][]*ast.TypeSpec, testRun bool) (map[string][]protected, []analysisError) {
	res := make(map[string][]protected)
	var errors []analysisError
	for _, f := range files {
		fileName := fset.Position(f.Pos()).Filename
		// Each source file consists of a package clause defining the package to which it
		// belongs (https://go.dev/ref/spec#Source_file_organization), hence safe to dereference.
		pkg := f.Name.Name
		commentMap := ast.NewCommentMap(fset, f, f.Comments)
		var protectedInFile []protected

		for node, commentMapGroups := range commentMap {
			// Filter out nodes that are not fields. The linter only works for a struct fields protected
			// by another field.
			field, ok := node.(*ast.Field)
			if !ok {
				continue
			}

			for _, commentGroup := range commentMapGroups {
				for _, c := range commentGroup.List {
					if !strings.Contains(c.Text, protectedBy) {
						continue
					}

					spec, err := getStructSpec(field, fileStructs[fileName])
					if err != nil {
						errors = append(errors, *err)
						continue
					}

					lock, err := getLockField(c, spec, testRun)
					if err != nil {
						errors = append(errors, *err)
						continue
					}

					p := protected{
						field:           field,
						lock:            lock,
						containerStruct: spec,
						file:            f,
						fset:            fset,
					}

					protectedInFile = append(protectedInFile, p)
				}
			}
		}
		if len(protectedInFile) > 0 {
			res[pkg] = append(res[pkg], protectedInFile...)
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

func getStructSpec(field ast.Node, structs []*ast.TypeSpec) (*ast.TypeSpec, *analysisError) {
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

func getStructs(files []*ast.File, fset *token.FileSet) map[string][]*ast.TypeSpec {
	structs := make(map[string][]*ast.TypeSpec)
	for _, f := range files {
		fileName := fset.Position(f.Pos()).Filename
		fileStructs := getFileStructs(f.Decls)
		structs[fileName] = fileStructs
	}

	return structs
}

func getFileStructs(decls []ast.Decl) []*ast.TypeSpec {
	var structs []*ast.TypeSpec
	for _, decl := range decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if typeSpec.Name == nil {
						continue
					}
					if _, ok := typeSpec.Type.(*ast.StructType); ok {
						structs = append(structs, typeSpec)
					}
				}
			}
		}
	}

	return structs
}

// getLockName returns the first word in the comment after "protected by" statement or error if the statement is not
// found or found more than once.
func getLockName(comment *ast.Comment, testRun bool) (string, *analysisError) {
	text := comment.Text
	if testRun {
		if idx := strings.Index(comment.Text, "// want \""); idx != -1 {
			text = text[:idx]
		}
	}

	cnt := strings.Count(text, protectedBy)
	if cnt != 1 {
		return "", &analysisError{
			msg: fmt.Sprintf("found %d %q in comment %q, expected exact one", cnt, protectedBy, text),
			pos: comment.Pos(),
		}
	}

	idx := strings.Index(text, protectedBy)
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
