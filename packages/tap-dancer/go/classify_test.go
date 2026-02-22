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
