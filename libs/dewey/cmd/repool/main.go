package main

import (
	"github.com/amarbel-llc/purse-first/libs/dewey/alfa/analyzers/repool"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(repool.Analyzer)
}
