package main

import (
	"github.com/amarbel-llc/purse-first/libs/dewey/internal/alfa/analyzer_repool"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(analyzer_repool.Analyzer)
}
