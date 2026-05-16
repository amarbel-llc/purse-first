package main

import (
	"github.com/amarbel-llc/purse-first/libs/dewey/alfa/analyzer_seqerror"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(analyzer_seqerror.Analyzer)
}
