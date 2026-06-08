package analyzer_testui_test

import (
	"testing"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/alfa/analyzer_testui"
	"golang.org/x/tools/go/analysis/analysistest"
)

func Test(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer_testui.Analyzer, "a")
}
