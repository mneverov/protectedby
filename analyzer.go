package main

import (
	"flag"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

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
