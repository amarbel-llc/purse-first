package tap

import "testing"

func TestParsePlan(t *testing.T) {
	tests := []struct {
		line    string
		count   int
		reason  string
		wantErr bool
	}{
		{"1..5", 5, "", false},
		{"1..0", 0, "", false},
		{"1..0 # skip all", 0, "skip all", false},
		{"1..100", 100, "", false},
	}
	for _, tt := range tests {
		p, err := parsePlan(tt.line)
		if (err != nil) != tt.wantErr {
			t.Errorf("parsePlan(%q) error = %v, wantErr %v", tt.line, err, tt.wantErr)
			continue
		}
		if err != nil {
			continue
		}
		if p.Count != tt.count {
			t.Errorf("parsePlan(%q).Count = %d, want %d", tt.line, p.Count, tt.count)
		}
		if p.Reason != tt.reason {
			t.Errorf("parsePlan(%q).Reason = %q, want %q", tt.line, p.Reason, tt.reason)
		}
	}
}

func TestParseTestPoint(t *testing.T) {
	tests := []struct {
		line      string
		ok        bool
		number    int
		desc      string
		directive Directive
		reason    string
	}{
		{"ok", true, 0, "", DirectiveNone, ""},
		{"ok 1", true, 1, "", DirectiveNone, ""},
		{"ok 1 - first test", true, 1, "first test", DirectiveNone, ""},
		{"not ok 2 - failing", false, 2, "failing", DirectiveNone, ""},
		{"ok 3 - skipped # SKIP not applicable", true, 3, "skipped", DirectiveSkip, "not applicable"},
		{"not ok 4 - todo # TODO not done", false, 4, "todo", DirectiveTodo, "not done"},
		{"ok - no number", true, 0, "no number", DirectiveNone, ""},
		{"not ok - also no number", false, 0, "also no number", DirectiveNone, ""},
		{"ok 1 - has \\# escaped hash", true, 1, "has # escaped hash", DirectiveNone, ""},
	}
	for _, tt := range tests {
		tp, _ := parseTestPoint(tt.line)
		if tp.OK != tt.ok {
			t.Errorf("parseTestPoint(%q).OK = %v, want %v", tt.line, tp.OK, tt.ok)
		}
		if tp.Number != tt.number {
			t.Errorf("parseTestPoint(%q).Number = %d, want %d", tt.line, tp.Number, tt.number)
		}
		if tp.Description != tt.desc {
			t.Errorf("parseTestPoint(%q).Description = %q, want %q", tt.line, tp.Description, tt.desc)
		}
		if tp.Directive != tt.directive {
			t.Errorf("parseTestPoint(%q).Directive = %v, want %v", tt.line, tp.Directive, tt.directive)
		}
		if tp.Reason != tt.reason {
			t.Errorf("parseTestPoint(%q).Reason = %q, want %q", tt.line, tp.Reason, tt.reason)
		}
	}
}

func TestParseBailOut(t *testing.T) {
	tests := []struct {
		line   string
		reason string
	}{
		{"Bail out!", ""},
		{"Bail out! database down", "database down"},
	}
	for _, tt := range tests {
		b := parseBailOut(tt.line)
		if b.Reason != tt.reason {
			t.Errorf("parseBailOut(%q).Reason = %q, want %q", tt.line, b.Reason, tt.reason)
		}
	}
}

func TestParsePragma(t *testing.T) {
	tests := []struct {
		line    string
		key     string
		enabled bool
	}{
		{"pragma +strict", "strict", true},
		{"pragma -strict", "strict", false},
	}
	for _, tt := range tests {
		p := parsePragma(tt.line)
		if p.Key != tt.key {
			t.Errorf("parsePragma(%q).Key = %q, want %q", tt.line, p.Key, tt.key)
		}
		if p.Enabled != tt.enabled {
			t.Errorf("parsePragma(%q).Enabled = %v, want %v", tt.line, p.Enabled, tt.enabled)
		}
	}
}

func TestParseTestPointEscaping(t *testing.T) {
	tests := []struct {
		line string
		desc string
	}{
		{`ok 1 - has \# hash`, "has # hash"},
		{`ok 1 - has \\ backslash`, `has \ backslash`},
		{`ok 1 - has \\\# both`, `has \# both`},
		{`ok 1 - normal desc`, "normal desc"},
	}
	for _, tt := range tests {
		tp, _ := parseTestPoint(tt.line)
		if tp.Description != tt.desc {
			t.Errorf("parseTestPoint(%q).Description = %q, want %q", tt.line, tp.Description, tt.desc)
		}
	}
}

func TestDirectiveCase(t *testing.T) {
	tests := []struct {
		line      string
		directive Directive
	}{
		{"ok 1 - x # SKIP reason", DirectiveSkip},
		{"ok 1 - x # skip reason", DirectiveSkip},
		{"ok 1 - x # Skip reason", DirectiveSkip},
		{"ok 1 - x # TODO reason", DirectiveTodo},
		{"ok 1 - x # todo reason", DirectiveTodo},
		{"ok 1 - x # Todo reason", DirectiveTodo},
	}
	for _, tt := range tests {
		tp, _ := parseTestPoint(tt.line)
		if tp.Directive != tt.directive {
			t.Errorf("parseTestPoint(%q).Directive = %v, want %v", tt.line, tp.Directive, tt.directive)
		}
	}
}
