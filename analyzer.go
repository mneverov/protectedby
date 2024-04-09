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

const protectedBy = "protected by "

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
	structs := getStructs(pass.Files, pass.Fset)
	protectedToLocks, err := parseComments(pass.Files, pass.Fset, structs)
	if err != nil {
		return nil, err
	}

	println(protectedToLocks)

	return nil, nil
}

func parseComments(files []*ast.File, fset *token.FileSet, fileStructs map[string][]*ast.TypeSpec) (map[string][]protected, error) {
	res := make(map[string][]protected)
	for _, f := range files {
		// full file name like /home/mneverov/go/src/github.com/mneverov/protectedby/testdata/src/protectedby/testfile.go
		fileName := fset.Position(f.Pos()).Filename
		commentMap := ast.NewCommentMap(fset, f, f.Comments)
		var protectedInFile []protected

		for node, commentMapGroups := range commentMap {
			// Filter out nodes that are not fields. The linter only works for a struct fields protected
			// by another field.
			fieldDecl, ok := node.(*ast.Field)
			if !ok {
				continue
			}

			for _, commentGroup := range commentMapGroups {
				for _, c := range commentGroup.List {
					if !strings.Contains(c.Text, protectedBy) {
						continue
					}

					spec, err := getStructSpec(fieldDecl, fileStructs[fileName])
					if err != nil {
						return nil, err
					}

					lockField, err := getLockField(c, spec)
					if err != nil {
						return nil, err
					}

					p := protected{
						field:           fieldDecl,
						lock:            lockField,
						containerStruct: spec,
						file:            f,
						fset:            fset,
					}

					protectedInFile = append(protectedInFile, p)
				}
			}
		}
		if len(protectedInFile) > 0 {
			res[fileName] = protectedInFile
		}
	}

	return res, nil
}

func getLockField(comment *ast.Comment, ts *ast.TypeSpec) (*ast.Field, error) {
	lockName, err := getLockName(comment.Text)
	if err != nil {
		return nil, err
	}

	st := ts.Type.(*ast.StructType)
	if st.Fields == nil {
		// Presence of the struct Name is checked in getStructSpec.
		return nil, fmt.Errorf("failed to find lock %q in struct %s: fieldList is nil", lockName, ts.Name.Name)
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
		return nil, fmt.Errorf("struct %q does not have lock field %q", ts.Name.Name, lockName)
	}

	return lockField, nil
}

func getStructSpec(field ast.Node, structs []*ast.TypeSpec) (*ast.TypeSpec, error) {
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
		return nil, fmt.Errorf("failed to find a struct for %s", field)
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
func getLockName(comment string) (string, error) {
	if cnt := strings.Count(comment, protectedBy); cnt != 1 {
		return "", fmt.Errorf("found %d %q in %q, expected exact one", cnt, protectedBy, comment)
	}

	idx := strings.Index(comment, protectedBy)
	if idx == -1 {
		return "", fmt.Errorf("comment %q does not contain %q statement", comment, protectedBy)
	}

	c := comment[idx+len(protectedBy):]
	fields := strings.FieldsFunc(c, isLetterOrNumber)
	if len(fields) == 0 {
		return "", fmt.Errorf("failed to parse lock name from comment %q", comment)
	}

	return fields[0], nil
}

func isLetterOrNumber(c rune) bool {
	return !unicode.IsLetter(c) && !unicode.IsNumber(c)
}
