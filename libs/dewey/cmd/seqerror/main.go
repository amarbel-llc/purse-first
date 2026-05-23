package main

import (
	"os"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/0/buildinfo"
	"github.com/amarbel-llc/purse-first/libs/dewey/internal/alfa/analyzer_seqerror"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		buildinfo.Print(os.Stdout, os.Args[0])
		return
	}
	singlechecker.Main(analyzer_seqerror.Analyzer)
}
