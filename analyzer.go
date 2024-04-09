package main

import (
	"flag"
	"go/ast"
	"go/token"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
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

func run(pass *analysis.Pass) (interface{}, error) {
	return nil, nil
}
