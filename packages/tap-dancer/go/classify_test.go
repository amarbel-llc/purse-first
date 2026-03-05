package tap

import "testing"

func TestClassifyVersion(t *testing.T) {
	tests := []struct {
		line string
		want lineKind
	}{
		{"TAP version 14", lineVersion},
		{"TAP version 13", lineUnknown},
		{"TAP version 14 ", lineUnknown},
		{"tap version 14", lineUnknown},
	}
	for _, tt := range tests {
		if got := classifyLine(tt.line); got != tt.want {
			t.Errorf("classifyLine(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestClassifyPlan(t *testing.T) {
	tests := []struct {
		line string
		want lineKind
	}{
		{"1..5", linePlan},
		{"1..0", linePlan},
		{"1..0 # skip all", linePlan},
		{"1..100", linePlan},
		{"2..5", lineUnknown},
		{"1..", lineUnknown},
	}
	for _, tt := range tests {
		if got := classifyLine(tt.line); got != tt.want {
			t.Errorf("classifyLine(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestClassifyTestPoint(t *testing.T) {
	tests := []struct {
		line string
		want lineKind
	}{
		{"ok", lineTestPoint},
		{"ok 1", lineTestPoint},
		{"ok 1 - description", lineTestPoint},
		{"not ok", lineTestPoint},
		{"not ok 2 - failing", lineTestPoint},
		{"ok 1 - desc # SKIP reason", lineTestPoint},
		{"not ok 3 - desc # TODO reason", lineTestPoint},
		{"okay", lineUnknown},
		{"not okay", lineUnknown},
	}
	for _, tt := range tests {
		if got := classifyLine(tt.line); got != tt.want {
			t.Errorf("classifyLine(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestClassifyYAMLMarkers(t *testing.T) {
	tests := []struct {
		line string
		want lineKind
	}{
		{"---", lineYAMLStart},
		{"...", lineYAMLEnd},
	}
	for _, tt := range tests {
		if got := classifyLine(tt.line); got != tt.want {
			t.Errorf("classifyLine(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestClassifyBailOut(t *testing.T) {
	tests := []struct {
		line string
		want lineKind
	}{
		{"Bail out!", lineBailOut},
		{"Bail out! reason", lineBailOut},
		{"bail out!", lineUnknown},
		{"Bail out", lineUnknown},
	}
	for _, tt := range tests {
		if got := classifyLine(tt.line); got != tt.want {
			t.Errorf("classifyLine(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestClassifyPragma(t *testing.T) {
	tests := []struct {
		line string
		want lineKind
	}{
		{"pragma +strict", linePragma},
		{"pragma -strict", linePragma},
		{"pragma strict", lineUnknown},
	}
	for _, tt := range tests {
		if got := classifyLine(tt.line); got != tt.want {
			t.Errorf("classifyLine(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ok 1 - plain", "ok 1 - plain"},
		{"\033[32mok\033[0m 1 - colored", "ok 1 - colored"},
		{"\033[31mnot ok\033[0m 2 - fail", "not ok 2 - fail"},
		{"\033[32mok\033[0m 1 - desc \033[33m# SKIP\033[0m reason", "ok 1 - desc # SKIP reason"},
		{"\033[31mBail out!\033[0m reason", "Bail out! reason"},
		// Non-SGR CSI sequences should also be stripped
		{"\033[2Jok 1 - after clear", "ok 1 - after clear"},
		{"no escapes", "no escapes"},
	}
	for _, tt := range tests {
		if got := stripANSI(tt.input); got != tt.want {
			t.Errorf("stripANSI(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestStripNonSGR(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// SGR sequences preserved
		{"\033[32mok\033[0m", "\033[32mok\033[0m"},
		{"\033[31;1mbold red\033[0m", "\033[31;1mbold red\033[0m"},
		// Non-SGR CSI sequences stripped
		{"\033[2Jcleared", "cleared"},
		{"\033[Hcursor home", "cursor home"},
		{"\033[3Aup three", "up three"},
		// Mixed: SGR preserved, non-SGR stripped
		{"\033[2J\033[31merror\033[0m text", "\033[31merror\033[0m text"},
		// Plain text unchanged
		{"no escapes", "no escapes"},
	}
	for _, tt := range tests {
		if got := stripNonSGR(tt.input); got != tt.want {
			t.Errorf("stripNonSGR(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestClassifyComment(t *testing.T) {
	tests := []struct {
		line string
		want lineKind
	}{
		{"# comment", lineComment},
		{"# Subtest: name", lineSubtestComment},
		{"#comment", lineComment},
	}
	for _, tt := range tests {
		if got := classifyLine(tt.line); got != tt.want {
			t.Errorf("classifyLine(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}
