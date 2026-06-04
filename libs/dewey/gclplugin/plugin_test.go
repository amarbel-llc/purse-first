package gclplugin

import (
	"testing"

	"github.com/golangci/plugin-module-register/register"
)

func ptr(b bool) *bool { return &b }

func TestNewDefaultsToAllAnalyzers(t *testing.T) {
	lp, err := New(nil)
	if err != nil {
		t.Fatalf("New(nil): %v", err)
	}

	analyzers, err := lp.BuildAnalyzers()
	if err != nil {
		t.Fatalf("BuildAnalyzers: %v", err)
	}

	// Only the default-on analyzers (defererr, seqerror, repool) appear;
	// actx is opt-in and stays out of the default set.
	if got, want := len(analyzers), 3; got != want {
		t.Fatalf("default analyzer count = %d, want %d", got, want)
	}

	if got := lp.GetLoadMode(); got != register.LoadModeTypesInfo {
		t.Fatalf("GetLoadMode() = %q, want %q", got, register.LoadModeTypesInfo)
	}
}

func TestSettingsSelectAnalyzers(t *testing.T) {
	cases := []struct {
		name     string
		settings map[string]any
		want     []string
	}{
		{
			name:     "repool disabled",
			settings: map[string]any{"repool": false},
			want:     []string{"defererr", "seqerror"},
		},
		{
			name:     "only defererr",
			settings: map[string]any{"defererr": true, "seqerror": false, "repool": false},
			want:     []string{"defererr"},
		},
		{
			name:     "all explicitly off",
			settings: map[string]any{"defererr": false, "seqerror": false, "repool": false},
			want:     nil,
		},
		{
			name:     "actx opt-in",
			settings: map[string]any{"actx": true},
			want:     []string{"defererr", "seqerror", "repool", "actx"},
		},
		{
			name:     "actx opt-in alone",
			settings: map[string]any{"defererr": false, "seqerror": false, "repool": false, "actx": true},
			want:     []string{"actx"},
		},
		{
			name:     "paramobj opt-in",
			settings: map[string]any{"paramobj": true},
			want:     []string{"defererr", "seqerror", "repool", "paramobj"},
		},
		{
			name:     "both opt-ins",
			settings: map[string]any{"actx": true, "paramobj": true},
			want:     []string{"defererr", "seqerror", "repool", "actx", "paramobj"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lp, err := New(tc.settings)
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			analyzers, err := lp.BuildAnalyzers()
			if err != nil {
				t.Fatalf("BuildAnalyzers: %v", err)
			}

			got := make([]string, len(analyzers))
			for i, a := range analyzers {
				got[i] = a.Name
			}

			if len(got) != len(tc.want) {
				t.Fatalf("analyzers = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("analyzers = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestNewRejectsUnknownSettings(t *testing.T) {
	if _, err := New(map[string]any{"bogus": true}); err == nil {
		t.Fatal("New with unknown field: expected error, got nil")
	}
}

func TestEnabledHelper(t *testing.T) {
	if !enabled(nil) {
		t.Fatal("enabled(nil) = false, want true")
	}
	if enabled(ptr(false)) {
		t.Fatal("enabled(false) = true, want false")
	}
	if !enabled(ptr(true)) {
		t.Fatal("enabled(true) = false, want true")
	}
}

func TestOptInHelper(t *testing.T) {
	if optIn(nil) {
		t.Fatal("optIn(nil) = true, want false")
	}
	if optIn(ptr(false)) {
		t.Fatal("optIn(false) = true, want false")
	}
	if !optIn(ptr(true)) {
		t.Fatal("optIn(true) = false, want true")
	}
}
