package seqerror_test

import (
	"testing"

	"github.com/amarbel-llc/purse-first/libs/dewey/alfa/analyzers/seqerror"
	"golang.org/x/tools/go/analysis/analysistest"
)

func Test(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, seqerror.Analyzer, "a")
}
