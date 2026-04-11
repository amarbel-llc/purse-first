package repool_test

import (
	"testing"

	"github.com/amarbel-llc/purse-first/libs/dewey/alfa/analyzers/repool"
	"golang.org/x/tools/go/analysis/analysistest"
)

func Test(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, repool.Analyzer, "a")
}
