package analyzer_actx_test

import (
	"testing"

	"code.linenisgreat.com/purse-first/libs/dewey/internal/alfa/analyzer_actx"
	"golang.org/x/tools/go/analysis/analysistest"
)

func Test(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer_actx.Analyzer, "a")
}
