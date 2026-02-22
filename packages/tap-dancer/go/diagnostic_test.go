package tap

import "testing"

func TestSeverityString(t *testing.T) {
	tests := []struct {
		s    Severity
		want string
	}{
		{SeverityError, "error"},
		{SeverityWarning, "warning"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("Severity(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestDirectiveString(t *testing.T) {
	tests := []struct {
		d    Directive
		want string
	}{
		{DirectiveNone, ""},
		{DirectiveSkip, "SKIP"},
		{DirectiveTodo, "TODO"},
	}
	for _, tt := range tests {
		if got := tt.d.String(); got != tt.want {
			t.Errorf("Directive(%d).String() = %q, want %q", tt.d, got, tt.want)
		}
	}
}
