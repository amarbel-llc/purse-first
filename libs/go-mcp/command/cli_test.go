package command

import (
	"context"
	"encoding/json"
	"testing"
)

func TestRunCLIDispatchesRun(t *testing.T) {
	var called bool
	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "greet",
		Params: []Param{
			{Name: "name", Type: String, Description: "Name to greet", Required: true},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			called = true
			var params struct {
				Name string `json:"name"`
			}
			json.Unmarshal(args, &params)
			return TextResult("hello " + params.Name), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"greet", "--name", "world"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if !called {
		t.Error("Run handler was not called")
	}
}

func TestRunCLIDispatchesRunCLI(t *testing.T) {
	var called bool
	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "open",
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			called = true
			return nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"open"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if !called {
		t.Error("RunCLI handler was not called")
	}
}

func TestRunCLIPrefersRunCLIOverRun(t *testing.T) {
	var ranCLI bool
	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "dual",
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			t.Error("Run should not be called when RunCLI is set")
			return TextResult(""), nil
		},
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			ranCLI = true
			return nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"dual"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if !ranCLI {
		t.Error("RunCLI handler was not called")
	}
}

func TestRunCLIBoolFlag(t *testing.T) {
	var got bool
	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		Params: []Param{
			{Name: "verbose", Type: Bool, Description: "Verbose output"},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Verbose bool `json:"verbose"`
			}
			json.Unmarshal(args, &params)
			got = params.Verbose
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"cmd", "--verbose"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if !got {
		t.Error("verbose should be true")
	}
}

func TestRunCLIIntFlag(t *testing.T) {
	var got int
	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		Params: []Param{
			{Name: "count", Type: Int, Description: "Count"},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Count int `json:"count"`
			}
			json.Unmarshal(args, &params)
			got = params.Count
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"cmd", "--count", "42"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if got != 42 {
		t.Errorf("count = %d, want 42", got)
	}
}

func TestRunCLIArrayFlag(t *testing.T) {
	var got []string
	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		Params: []Param{
			{Name: "tags", Type: Array, Description: "Tags"},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Tags []string `json:"tags"`
			}
			json.Unmarshal(args, &params)
			got = params.Tags
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"cmd", "--tags", "a", "--tags", "b"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("tags = %v, want [a b]", got)
	}
}

func TestRunCLIGlobalParams(t *testing.T) {
	var format string
	app := NewApp("test", "test app")
	app.Params = []Param{
		{Name: "format", Type: String, Description: "Output format"},
	}
	app.AddCommand(&Command{
		Name: "status",
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Format string `json:"format"`
			}
			json.Unmarshal(args, &params)
			format = params.Format
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"--format", "tap", "status"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if format != "tap" {
		t.Errorf("format = %q, want %q", format, "tap")
	}
}

func TestRunCLIUnknownCommand(t *testing.T) {
	app := NewApp("test", "test app")
	err := app.RunCLI(context.Background(), []string{"nonexistent"}, StubPrompter{})
	if err == nil {
		t.Error("expected error for unknown command")
	}
}

func TestRunCLIEqualsFlag(t *testing.T) {
	var got string
	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		Params: []Param{
			{Name: "name", Type: String, Description: "Name"},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Name string `json:"name"`
			}
			json.Unmarshal(args, &params)
			got = params.Name
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"cmd", "--name=alice"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if got != "alice" {
		t.Errorf("name = %q, want %q", got, "alice")
	}
}

func TestRunCLIGlobalParamsAfterSubcommand(t *testing.T) {
	var format string
	app := NewApp("test", "test app")
	app.Params = []Param{
		{Name: "format", Type: String, Description: "Output format"},
	}
	app.AddCommand(&Command{
		Name: "status",
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Format string `json:"format"`
			}
			json.Unmarshal(args, &params)
			format = params.Format
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"status", "--format", "tap"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if format != "tap" {
		t.Errorf("format = %q, want %q", format, "tap")
	}
}

func TestRunCLIPositionalArg(t *testing.T) {
	var got string
	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "open",
		Params: []Param{
			{Name: "target", Type: String, Description: "target path"},
			{Name: "verbose", Type: Bool, Description: "verbose output"},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Target string `json:"target"`
			}
			json.Unmarshal(args, &params)
			got = params.Target
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"open", "eng/worktrees/repo/branch"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if got != "eng/worktrees/repo/branch" {
		t.Errorf("target = %q, want %q", got, "eng/worktrees/repo/branch")
	}
}

func TestRunCLIPositionalArgWithFlags(t *testing.T) {
	var target, format string
	app := NewApp("test", "test app")
	app.Params = []Param{
		{Name: "format", Type: String, Description: "Output format"},
	}
	app.AddCommand(&Command{
		Name: "open",
		Params: []Param{
			{Name: "target", Type: String, Description: "target path"},
			{Name: "no-attach", Type: Bool, Description: "skip attach"},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Target string `json:"target"`
				Format string `json:"format"`
			}
			json.Unmarshal(args, &params)
			target = params.Target
			format = params.Format
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"open", "eng/worktrees/repo/branch", "--format", "tap"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if target != "eng/worktrees/repo/branch" {
		t.Errorf("target = %q, want %q", target, "eng/worktrees/repo/branch")
	}
	if format != "tap" {
		t.Errorf("format = %q, want %q", format, "tap")
	}
}

func TestRunCLIPrefixSubcommand(t *testing.T) {
	var called bool
	app := NewApp("test", "test app")

	sub := NewApp("perms", "Manage permissions")
	sub.AddCommand(&Command{
		Name: "check",
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			called = true
			return TextResult("ok"), nil
		},
	})
	app.MergeWithPrefix(sub, "perms")

	err := app.RunCLI(context.Background(), []string{"perms", "check"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if !called {
		t.Error("perms-check handler was not called")
	}
}

func TestRunCLIShortBoolFlag(t *testing.T) {
	var got bool
	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		Params: []Param{
			{Name: "verbose", Type: Bool, Description: "Verbose output", Short: 'v'},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Verbose bool `json:"verbose"`
			}
			json.Unmarshal(args, &params)
			got = params.Verbose
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"cmd", "-v"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if !got {
		t.Error("verbose should be true when using -v")
	}
}

func TestRunCLIShortStringFlag(t *testing.T) {
	var got string
	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		Params: []Param{
			{Name: "name", Type: String, Description: "Name", Short: 'n'},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Name string `json:"name"`
			}
			json.Unmarshal(args, &params)
			got = params.Name
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"cmd", "-n", "alice"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if got != "alice" {
		t.Errorf("name = %q, want %q", got, "alice")
	}
}

func TestRunCLIShortFlagEquals(t *testing.T) {
	var got string
	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		Params: []Param{
			{Name: "name", Type: String, Description: "Name", Short: 'n'},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Name string `json:"name"`
			}
			json.Unmarshal(args, &params)
			got = params.Name
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"cmd", "-n=bob"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if got != "bob" {
		t.Errorf("name = %q, want %q", got, "bob")
	}
}

func TestRunCLIShortIntFlag(t *testing.T) {
	var got int
	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		Params: []Param{
			{Name: "count", Type: Int, Description: "Count", Short: 'c'},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Count int `json:"count"`
			}
			json.Unmarshal(args, &params)
			got = params.Count
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"cmd", "-c", "7"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if got != 7 {
		t.Errorf("count = %d, want 7", got)
	}
}

func TestRunCLIShortArrayFlag(t *testing.T) {
	var got []string
	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		Params: []Param{
			{Name: "tags", Type: Array, Description: "Tags", Short: 't'},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Tags []string `json:"tags"`
			}
			json.Unmarshal(args, &params)
			got = params.Tags
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"cmd", "-t", "a", "-t", "b"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("tags = %v, want [a b]", got)
	}
}

func TestRunCLIShortAndLongFlagsMixed(t *testing.T) {
	var name string
	var verbose bool
	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		Params: []Param{
			{Name: "name", Type: String, Description: "Name", Short: 'n'},
			{Name: "verbose", Type: Bool, Description: "Verbose", Short: 'v'},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Name    string `json:"name"`
				Verbose bool   `json:"verbose"`
			}
			json.Unmarshal(args, &params)
			name = params.Name
			verbose = params.Verbose
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"cmd", "-v", "--name", "alice"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if !verbose {
		t.Error("verbose should be true")
	}
	if name != "alice" {
		t.Errorf("name = %q, want %q", name, "alice")
	}
}

func TestRunCLIShortFlagGlobal(t *testing.T) {
	var format string
	app := NewApp("test", "test app")
	app.Params = []Param{
		{Name: "format", Type: String, Description: "Output format", Short: 'f'},
	}
	app.AddCommand(&Command{
		Name: "status",
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Format string `json:"format"`
			}
			json.Unmarshal(args, &params)
			format = params.Format
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"-f", "tap", "status"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if format != "tap" {
		t.Errorf("format = %q, want %q", format, "tap")
	}
}

func TestRunCLIShortFlagUnknownPassedThrough(t *testing.T) {
	var got string
	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		Params: []Param{
			{Name: "target", Type: String, Description: "Target"},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Target string `json:"target"`
			}
			json.Unmarshal(args, &params)
			got = params.Target
			return TextResult(""), nil
		},
	})

	// -x is unknown, should be treated as positional
	err := app.RunCLI(context.Background(), []string{"cmd", "-x"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if got != "-x" {
		t.Errorf("target = %q, want %q", got, "-x")
	}
}

func TestDuplicateShortFlagsPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate short flags")
		}
	}()

	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		Params: []Param{
			{Name: "verbose", Type: Bool, Description: "Verbose", Short: 'v'},
			{Name: "version", Type: Bool, Description: "Version", Short: 'v'},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			return TextResult(""), nil
		},
	})

	// Force validation by trying to use the command
	app.RunCLI(context.Background(), []string{"cmd"}, StubPrompter{})
}

func TestShortFlagNotInJSONSchema(t *testing.T) {
	cmd := &Command{
		Name: "status",
		Params: []Param{
			{Name: "verbose", Type: Bool, Description: "Verbose output", Short: 'v'},
		},
	}

	schema := cmd.InputSchema()
	var parsed map[string]any
	json.Unmarshal(schema, &parsed)

	props := parsed["properties"].(map[string]any)
	verboseProp := props["verbose"].(map[string]any)

	if _, exists := verboseProp["short"]; exists {
		t.Error("Short field should not appear in JSON schema")
	}
}
