package main

import (
	"os"

	"code.linenisgreat.com/purse-first/libs/dewey/internal/0/analyzer_repool"
	"code.linenisgreat.com/purse-first/libs/dewey/internal/0/buildinfo"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		buildinfo.Print(os.Stdout, os.Args[0])
		return
	}
	singlechecker.Main(analyzer_repool.Analyzer)
}
