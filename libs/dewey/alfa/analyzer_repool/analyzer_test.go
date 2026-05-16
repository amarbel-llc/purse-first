package analyzer_repool_test

import (
	"testing"

	"github.com/amarbel-llc/purse-first/libs/dewey/alfa/analyzer_repool"
	"golang.org/x/tools/go/analysis/analysistest"
)

func Test(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer_repool.Analyzer, "a")
}
