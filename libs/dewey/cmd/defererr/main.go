package main

import (
	"github.com/amarbel-llc/purse-first/libs/dewey/alfa/analyzers/defererr"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(defererr.Analyzer)
}
