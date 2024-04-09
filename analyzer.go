package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"strings"
	"unicode"
)

const protectedBy = "protected by "

type protected struct {
	field           *ast.Field
	lock            *ast.Field
	containerStruct *ast.StructType
	file            *ast.File
	fset            *token.FileSet
}

var analyzer = &analysis.Analyzer{
	Name:     "protectedby",
	Doc:      "Checks concurrent access to shared resources.",
	Flags:    flag.FlagSet{},
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
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

func run(pass *analysis.Pass) (interface{}, error) {
	return nil, nil
}
