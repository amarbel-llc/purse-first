// Package gclplugin registers the dewey static analyzers as a
// golangci-lint Module Plugin.
//
// It wires the three dewey go/analysis analyzers — defererr, seqerror,
// and repool — into golangci-lint's module plugin system so a downstream
// repo that already gates on golangci-lint can fold them into that same
// run instead of a separate `go vet -vettool` pass.
//
// # Consuming the plugin
//
// Add the module to a .custom-gcl.yml:
//
//	version: v2.5.0
//	plugins:
//	  - module: github.com/amarbel-llc/purse-first/libs/dewey
//	    import: github.com/amarbel-llc/purse-first/libs/dewey/gclplugin
//	    version: latest
//
// Build a custom binary with `golangci-lint custom`, then enable the
// linter in .golangci.yml:
//
//	linters:
//	  enable:
//	    - dewey
//	  settings:
//	    custom:
//	      dewey:
//	        type: module
//	        description: dewey static analyzers (defererr, seqerror, repool)
//	        settings:
//	          defererr: true
//	          seqerror: true
//	          repool: true
//
// All three analyzers are enabled by default; the settings block lets a
// consumer opt out per analyzer. defererr and seqerror are general Go
// checks; repool is purse-first-pool-specific and a no-op in packages
// that do not import the pool ownership types.
//
// The analyzers' suppression comments (//defer:err-checked,
// //seq:err-checked, //repool:owned, //repool:suppress) carry over
// unchanged.
package gclplugin

import (
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/amarbel-llc/purse-first/libs/dewey/pkgs/analyzer_defererr"
	"github.com/amarbel-llc/purse-first/libs/dewey/pkgs/analyzer_repool"
	"github.com/amarbel-llc/purse-first/libs/dewey/pkgs/analyzer_seqerror"
)

func init() {
	register.Plugin("dewey", New)
}

// Settings selects which dewey analyzers the plugin contributes. A nil
// pointer means "use the default" (enabled); an explicit false disables
// that analyzer. The zero Settings value therefore enables all three.
type Settings struct {
	Defererr *bool `json:"defererr,omitempty"`
	Seqerror *bool `json:"seqerror,omitempty"`
	Repool   *bool `json:"repool,omitempty"`
}

// Plugin is the dewey golangci-lint module plugin.
type Plugin struct {
	settings Settings
}

// New is the plugin constructor registered with golangci-lint.
func New(conf any) (register.LinterPlugin, error) {
	settings, err := register.DecodeSettings[Settings](conf)
	if err != nil {
		return nil, err
	}

	return &Plugin{settings: settings}, nil
}

// BuildAnalyzers returns the enabled dewey analyzers.
func (p *Plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	analyzers := make([]*analysis.Analyzer, 0, 3)

	if enabled(p.settings.Defererr) {
		analyzers = append(analyzers, analyzer_defererr.Analyzer)
	}
	if enabled(p.settings.Seqerror) {
		analyzers = append(analyzers, analyzer_seqerror.Analyzer)
	}
	if enabled(p.settings.Repool) {
		analyzers = append(analyzers, analyzer_repool.Analyzer)
	}

	return analyzers, nil
}

// GetLoadMode reports that the analyzers need full type information; all
// three inspect go/types data.
func (p *Plugin) GetLoadMode() string {
	return register.LoadModeTypesInfo
}

// enabled reports whether an analyzer toggle is on. A nil pointer
// defaults to enabled so the zero Settings value enables every analyzer.
func enabled(b *bool) bool {
	return b == nil || *b
}
