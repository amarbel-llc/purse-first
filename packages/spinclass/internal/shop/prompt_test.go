package shop

import "testing"

func TestDirtyActionStringValues(t *testing.T) {
	tests := []struct {
		action dirtyAction
		want   string
	}{
		{actionDiscard, "Discard changes and merge"},
		{actionReattach, "Reattach to session"},
		{actionExit, "Exit without integrating"},
	}
	for _, tt := range tests {
		if got := string(tt.action); got != tt.want {
			t.Errorf("dirtyAction = %q, want %q", got, tt.want)
		}
	}
}
