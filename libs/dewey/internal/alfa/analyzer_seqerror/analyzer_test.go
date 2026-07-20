package analyzer_seqerror_test

import (
	"testing"

	"code.linenisgreat.com/purse-first/libs/dewey/internal/alfa/analyzer_seqerror"
	"golang.org/x/tools/go/analysis/analysistest"
)

func Test(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer_seqerror.Analyzer, "a")
}
