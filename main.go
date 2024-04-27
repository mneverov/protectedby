package main

import (
	"github.com/mneverov/protectedby/protectedby"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(protectedby.Analyzer)
}
