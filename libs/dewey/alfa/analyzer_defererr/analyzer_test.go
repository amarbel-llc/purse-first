package analyzer_defererr_test

import (
	"testing"

	"github.com/amarbel-llc/purse-first/libs/dewey/alfa/analyzer_defererr"
	"golang.org/x/tools/go/analysis/analysistest"
)

func Test(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer_defererr.Analyzer, "a")
}
