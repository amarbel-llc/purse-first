// Package gclplugin registers the dewey static analyzers as a
// golangci-lint Module Plugin.
//
// It wires the dewey go/analysis analyzers — defererr, seqerror,
// repool, and testui (enabled by default) plus actx and paramobj
// (opt-in) — into golangci-lint's module plugin system so a downstream
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
// Build a custom binary with `golangci-lint custom` — or skip that and
// consume this repo's flake output `golangci-lint-dewey`, a nix-built
// custom binary with the plugin already linked (see
// cmd/golangci-lint-dewey, purse-first#134). Then enable the linter in
// .golangci.yml:
//
//	linters:
//	  enable:
//	    - dewey
//	  settings:
//	    custom:
//	      dewey:
//	        type: module
//	        description: dewey static analyzers (defererr, seqerror, repool, testui)
//	        settings:
//	          defererr: true
//	          seqerror: true
//	          repool: true
//	          testui: true
//	          actx: true
//	          paramobj: true
//
// defererr, seqerror, repool, and testui are enabled by default; the
// settings block lets a consumer opt out per analyzer. defererr and
// seqerror are general Go checks; repool is purse-first-pool-specific
// and a no-op in packages that do not import the pool ownership types;
// testui (enforce dewey's test_ui.T over stdlib *testing.T) is a no-op
// outside test files. actx (enforce dewey's ActiveContext over stdlib
// context.Context) and paramobj (suggest parameter-object structs) are
// opt-in — disabled unless set to true — because they surface
// ecosystem-specific opinions rather than general Go correctness checks.
//
// The analyzers' suppression comments (//defer:err-checked,
// //seq:err-checked, //repool:owned, //repool:suppress, //testui:allow,
// //actx:allow, //paramobj:allow) carry over unchanged.
package gclplugin

import (
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/amarbel-llc/purse-first/libs/dewey/pkgs/analyzer_actx"
	"github.com/amarbel-llc/purse-first/libs/dewey/pkgs/analyzer_defererr"
	"github.com/amarbel-llc/purse-first/libs/dewey/pkgs/analyzer_paramobj"
	"github.com/amarbel-llc/purse-first/libs/dewey/pkgs/analyzer_repool"
	"github.com/amarbel-llc/purse-first/libs/dewey/pkgs/analyzer_seqerror"
	"github.com/amarbel-llc/purse-first/libs/dewey/pkgs/analyzer_testui"
)

func init() {
	register.Plugin("dewey", New)
}

// Settings selects which dewey analyzers the plugin contributes. For
// the default-on analyzers a nil pointer means "use the default"
// (enabled) and an explicit false disables them; for opt-in analyzers a
// nil pointer leaves them disabled. The zero Settings value therefore
// enables the three default-on analyzers and leaves actx off.
type Settings struct {
	Defererr *bool `json:"defererr,omitempty"`
	Seqerror *bool `json:"seqerror,omitempty"`
	Repool   *bool `json:"repool,omitempty"`
	// Testui is default-on: a nil pointer enables it. Set to false to
	// disable enforcing dewey's test_ui.T over stdlib *testing.T. It is a
	// no-op outside test files.
	Testui *bool `json:"testui,omitempty"`
	// Actx is opt-in: a nil pointer leaves it disabled. Set to true to
	// enforce dewey's ActiveContext over stdlib context.Context.
	Actx *bool `json:"actx,omitempty"`
	// Paramobj is opt-in: a nil pointer leaves it disabled. Set to true
	// to surface clusters of functions sharing a parameter list as
	// parameter-struct candidates.
	Paramobj *bool `json:"paramobj,omitempty"`
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
	analyzers := make([]*analysis.Analyzer, 0, 6)

	if enabled(p.settings.Defererr) {
		analyzers = append(analyzers, analyzer_defererr.Analyzer)
	}
	if enabled(p.settings.Seqerror) {
		analyzers = append(analyzers, analyzer_seqerror.Analyzer)
	}
	if enabled(p.settings.Repool) {
		analyzers = append(analyzers, analyzer_repool.Analyzer)
	}
	if enabled(p.settings.Testui) {
		analyzers = append(analyzers, analyzer_testui.Analyzer)
	}
	if optIn(p.settings.Actx) {
		analyzers = append(analyzers, analyzer_actx.Analyzer)
	}
	if optIn(p.settings.Paramobj) {
		analyzers = append(analyzers, analyzer_paramobj.Analyzer)
	}

	return analyzers, nil
}

// GetLoadMode reports that the analyzers need full type information;
// every dewey analyzer inspects go/types data.
func (p *Plugin) GetLoadMode() string {
	return register.LoadModeTypesInfo
}

// enabled reports whether an analyzer toggle is on. A nil pointer
// defaults to enabled so the zero Settings value enables the
// default-on analyzers.
func enabled(b *bool) bool {
	return b == nil || *b
}

// optIn reports whether an opt-in analyzer toggle is explicitly on. A
// nil pointer leaves it disabled so the zero Settings value does not
// surface ecosystem-specific opinions.
func optIn(b *bool) bool {
	return b != nil && *b
}
