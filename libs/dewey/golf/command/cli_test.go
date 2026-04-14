package command

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunCLIDispatchesRun(t *testing.T) {
	var called bool
	app := NewUtility("test", "test app")
	app.AddCommand(&Command{
		Name: "greet",
		OldParams: []OldParam{
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
	app := NewUtility("test", "test app")
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
	app := NewUtility("test", "test app")
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
	app := NewUtility("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		OldParams: []OldParam{
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
	app := NewUtility("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		OldParams: []OldParam{
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
	app := NewUtility("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		OldParams: []OldParam{
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
	app := NewUtility("test", "test app")
	app.OldParams = []OldParam{
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
	app := NewUtility("test", "test app")
	err := app.RunCLI(context.Background(), []string{"nonexistent"}, StubPrompter{})
	if err == nil {
		t.Error("expected error for unknown command")
	}
}

func TestRunCLIEqualsFlag(t *testing.T) {
	var got string
	app := NewUtility("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		OldParams: []OldParam{
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
	app := NewUtility("test", "test app")
	app.OldParams = []OldParam{
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
	app := NewUtility("test", "test app")
	app.AddCommand(&Command{
		Name: "open",
		OldParams: []OldParam{
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
	app := NewUtility("test", "test app")
	app.OldParams = []OldParam{
		{Name: "format", Type: String, Description: "Output format"},
	}
	app.AddCommand(&Command{
		Name: "open",
		OldParams: []OldParam{
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

	// Flags must precede positional args (POSIX convention).
	err := app.RunCLI(context.Background(), []string{"open", "--format", "tap", "eng/worktrees/repo/branch"}, StubPrompter{})
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
	app := NewUtility("test", "test app")

	sub := NewUtility("perms", "Manage permissions")
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
	app := NewUtility("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		OldParams: []OldParam{
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
	app := NewUtility("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		OldParams: []OldParam{
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
	app := NewUtility("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		OldParams: []OldParam{
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
	app := NewUtility("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		OldParams: []OldParam{
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
	app := NewUtility("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		OldParams: []OldParam{
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
	app := NewUtility("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		OldParams: []OldParam{
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
	app := NewUtility("test", "test app")
	app.OldParams = []OldParam{
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
	app := NewUtility("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		OldParams: []OldParam{
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

	app := NewUtility("test", "test app")
	// Panic should fire at AddCommand, not at RunCLI.
	app.AddCommand(&Command{
		Name: "cmd",
		OldParams: []OldParam{
			{Name: "verbose", Type: Bool, Description: "Verbose", Short: 'v'},
			{Name: "version", Type: Bool, Description: "Version", Short: 'v'},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			return TextResult(""), nil
		},
	})
}

func TestShortFlagCollisionGlobalAndCommandPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic when command param short flag conflicts with global param")
		}
		msg, ok := r.(string)
		if !ok {
			t.Errorf("panic value is not a string: %v", r)
			return
		}
		if !strings.Contains(msg, "conflicts with global param") {
			t.Errorf("unexpected panic message: %s", msg)
		}
	}()

	app := NewUtility("test", "test app")
	app.OldParams = []OldParam{
		{Name: "format", Type: String, Description: "Output format", Short: 'f'},
	}
	// This should panic at AddCommand because -f is already used by global param.
	app.AddCommand(&Command{
		Name: "cmd",
		OldParams: []OldParam{
			{Name: "file", Type: String, Description: "File path", Short: 'f'},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			return TextResult(""), nil
		},
	})
}

// TestBundledShortFlagsArePositional documents that multi-character short flag
// args like -vf (bundled flags) are intentionally treated as positional args.
// This is an accepted design choice: we only recognize single-character short
// flags, so -vf is not expanded into -v -f.
func TestBundledShortFlagsArePositional(t *testing.T) {
	var got string
	app := NewUtility("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		OldParams: []OldParam{
			{Name: "target", Type: String, Description: "Target"},
			{Name: "verbose", Type: Bool, Description: "Verbose", Short: 'v'},
			{Name: "force", Type: Bool, Description: "Force", Short: 'f'},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Target  string `json:"target"`
				Verbose bool   `json:"verbose"`
				Force   bool   `json:"force"`
			}
			json.Unmarshal(args, &params)
			got = params.Target
			if params.Verbose {
				t.Error("verbose should not be set by bundled -vf")
			}
			if params.Force {
				t.Error("force should not be set by bundled -vf")
			}
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"cmd", "-vf"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if got != "-vf" {
		t.Errorf("target = %q, want %q (bundled flags should be positional)", got, "-vf")
	}
}

func TestRunCLIShortFloatFlag(t *testing.T) {
	var got float64
	app := NewUtility("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		OldParams: []OldParam{
			{Name: "ratio", Type: Float, Description: "Ratio", Short: 'r'},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Ratio float64 `json:"ratio"`
			}
			json.Unmarshal(args, &params)
			got = params.Ratio
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"cmd", "-r", "3.14"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if got != 3.14 {
		t.Errorf("ratio = %v, want 3.14", got)
	}
}

func TestRunCLISingleDashLongBoolFlag(t *testing.T) {
	var got bool
	app := NewUtility("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		OldParams: []OldParam{
			{Name: "skip-empty", Type: Bool, Description: "Skip empty"},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				SkipEmpty bool `json:"skip-empty"`
			}
			json.Unmarshal(args, &params)
			got = params.SkipEmpty
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"cmd", "-skip-empty"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if !got {
		t.Error("skip-empty should be true when using -skip-empty")
	}
}

func TestRunCLISingleDashLongStringFlag(t *testing.T) {
	var got string
	app := NewUtility("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		OldParams: []OldParam{
			{Name: "output-dir", Type: String, Description: "Output directory"},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				OutputDir string `json:"output-dir"`
			}
			json.Unmarshal(args, &params)
			got = params.OutputDir
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"cmd", "-output-dir", "/tmp/out"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if got != "/tmp/out" {
		t.Errorf("output-dir = %q, want %q", got, "/tmp/out")
	}
}

func TestRunCLISingleDashLongEqualsFlag(t *testing.T) {
	var got string
	app := NewUtility("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		OldParams: []OldParam{
			{Name: "output-dir", Type: String, Description: "Output directory"},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				OutputDir string `json:"output-dir"`
			}
			json.Unmarshal(args, &params)
			got = params.OutputDir
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"cmd", "-output-dir=/tmp/out"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if got != "/tmp/out" {
		t.Errorf("output-dir = %q, want %q", got, "/tmp/out")
	}
}

func TestRunCLIHelpFlag(t *testing.T) {
	app := NewUtility("myapp", "My application")
	app.Description.Long = "A longer description of my application."
	app.AddCommand(&Command{
		Name:        "serve",
		Description: Description{Short: "Start the server"},
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			t.Error("serve handler should not be called when -h is passed at app level")
			return nil
		},
	})

	tests := []struct {
		name string
		args []string
	}{
		{"dash h", []string{"-h"}},
		{"double dash help", []string{"--help"}},
		{"help command", []string{"help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := app.RunCLI(context.Background(), tt.args, StubPrompter{})
			if err != nil {
				t.Errorf("RunCLI(%v): %v", tt.args, err)
			}
		})
	}
}

func TestRunCLICommandHelpFlag(t *testing.T) {
	app := NewUtility("myapp", "My application")
	app.AddCommand(&Command{
		Name: "serve",
		Description: Description{
			Short: "Start the server",
			Long:  "Start the server and listen on stdin/stdout.",
		},
		OldParams: []OldParam{
			{Name: "port", Type: Int, Description: "Port to listen on", Short: 'p'},
			{Name: "verbose", Type: Bool, Description: "Verbose output"},
		},
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			t.Error("serve handler should not be called when -h is passed")
			return nil
		},
	})

	tests := []struct {
		name string
		args []string
	}{
		{"command dash h", []string{"serve", "-h"}},
		{"command double dash help", []string{"serve", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := app.RunCLI(context.Background(), tt.args, StubPrompter{})
			if err != nil {
				t.Errorf("RunCLI(%v): %v", tt.args, err)
			}
		})
	}
}

func TestRunCLIPrefixSubcommandHelpFlag(t *testing.T) {
	app := NewUtility("myapp", "My application")

	sub := NewUtility("mcp", "MCP server")
	sub.AddCommand(&Command{
		Name:        "stdio",
		Description: Description{Short: "MCP over stdio"},
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			t.Error("mcp stdio handler should not be called when -h is passed")
			return nil
		},
	})
	app.MergeWithPrefix(sub, "mcp")

	tests := []struct {
		name string
		args []string
	}{
		{"prefix command dash h", []string{"mcp", "stdio", "-h"}},
		{"prefix command double dash help", []string{"mcp", "stdio", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := app.RunCLI(context.Background(), tt.args, StubPrompter{})
			if err != nil {
				t.Errorf("RunCLI(%v): %v", tt.args, err)
			}
		})
	}
}

func TestRunCLIPassthroughArgs(t *testing.T) {
	var got []string
	app := NewUtility("test", "test app")
	app.AddCommand(&Command{
		Name:            "exec-claude",
		Description:     Description{Short: "Execute claude with args"},
		PassthroughArgs: true,
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			var params struct {
				Args []string `json:"args"`
			}
			json.Unmarshal(args, &params)
			got = params.Args
			return nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"exec-claude", "hello", "world"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if len(got) != 2 || got[0] != "hello" || got[1] != "world" {
		t.Errorf("args = %v, want [hello world]", got)
	}
}

func TestRunCLIPassthroughArgsWithDashes(t *testing.T) {
	var got []string
	app := NewUtility("test", "test app")
	app.AddCommand(&Command{
		Name:            "exec-claude",
		Description:     Description{Short: "Execute claude with args"},
		PassthroughArgs: true,
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			var params struct {
				Args []string `json:"args"`
			}
			json.Unmarshal(args, &params)
			got = params.Args
			return nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"exec-claude", "--model", "opus", "-v", "--flag=value"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	expected := []string{"--model", "opus", "-v", "--flag=value"}
	if len(got) != len(expected) {
		t.Fatalf("args length = %d, want %d", len(got), len(expected))
	}
	for i, v := range expected {
		if got[i] != v {
			t.Errorf("args[%d] = %q, want %q", i, got[i], v)
		}
	}
}

func TestRunCLIPassthroughArgsEmpty(t *testing.T) {
	var got []string
	app := NewUtility("test", "test app")
	app.AddCommand(&Command{
		Name:            "exec-claude",
		Description:     Description{Short: "Execute claude with args"},
		PassthroughArgs: true,
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			var params struct {
				Args []string `json:"args"`
			}
			json.Unmarshal(args, &params)
			got = params.Args
			return nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"exec-claude"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("args = %v, want empty", got)
	}
}

// --- AddCmd + RunCLI integration tests ---
// These tests verify that commands registered via AddCmd (using the new Param
// API) work correctly through RunCLI's flag parsing and positional mapping.

func TestRunCLIWithParamArgPositionalOnly(t *testing.T) {
	app := NewUtility("test", "test app")

	var gotArg string
	app.AddCmd("open", &captureParamCmd{
		params: []Param{
			StringArg{Name: "path", Description: "File path", Required: true},
		},
		onRun: func(req Request) {
			gotArg = req.PopArg("path")
		},
	})

	err := app.RunCLI(t.Context(), []string{"open", "/tmp/foo"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI error: %v", err)
	}
	if gotArg != "/tmp/foo" {
		t.Errorf("PopArg(path) = %q, want %q", gotArg, "/tmp/foo")
	}
}

func TestRunCLIWithParamFlagsBeforeArg(t *testing.T) {
	app := NewUtility("test", "test app")

	var gotArg, gotFlag string
	app.AddCmd("init", &captureParamCmd{
		params: []Param{
			StringFlag{Name: "encryption", Description: "Encryption type"},
			StringArg{Name: "store-id", Description: "Store ID", Required: true},
		},
		onRun: func(req Request) {
			gotFlag = req.PopArg("encryption")
			gotArg = req.PopArg("store-id")
		},
	})

	err := app.RunCLI(t.Context(), []string{"init", "-encryption", "none", ".default"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI error: %v", err)
	}
	if gotFlag != "none" {
		t.Errorf("PopArg(encryption) = %q, want %q", gotFlag, "none")
	}
	if gotArg != ".default" {
		t.Errorf("PopArg(store-id) = %q, want %q", gotArg, ".default")
	}
}

func TestRunCLIWithParamArgBeforeFlagsIgnoresTrailingFlags(t *testing.T) {
	// Flags after positional args are treated as positional (POSIX convention).
	app := NewUtility("test", "test app")

	var gotJSON json.RawMessage
	app.AddCommand(&Command{
		Name: "init",
		Params: []Param{
			StringFlag{Name: "encryption", Description: "Encryption type"},
			StringArg{Name: "store-id", Description: "Store ID", Required: true},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			gotJSON = args
			return nil, nil
		},
	})

	// ".default" is a non-flag token, so "-encryption" and "none" after it
	// are treated as positional, not flags. encryption stays unset.
	err := app.RunCLI(t.Context(), []string{"init", ".default", "-encryption", "none"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI error: %v", err)
	}

	var vals map[string]any
	json.Unmarshal(gotJSON, &vals)

	if _, set := vals["encryption"]; set {
		t.Errorf("encryption should not be set (flags after positional are not parsed), got %v", vals["encryption"])
	}
	if vals["store-id"] != ".default" {
		t.Errorf("store-id = %v, want %q", vals["store-id"], ".default")
	}
}

// --- Flags-before-positionals enforcement (issue #39) ---

func TestRunCLIFlagsBeforeArgExactReproduction(t *testing.T) {
	// Exact reproduction from #39: two flags (string + bool=value) before positional.
	app := NewUtility("test", "test app")

	var gotArg, gotEncryption string
	var gotLock bool
	app.AddCmd("init", &captureParamCmd{
		params: []Param{
			StringFlag{Name: "encryption", Description: "Encryption type"},
			BoolFlag{Name: "lock-internal-files", Description: "Lock internal files"},
			StringArg{Name: "store-id", Description: "Store ID", Required: true},
		},
		onRun: func(req Request) {
			gotEncryption = req.PopArg("encryption")
			gotLock = req.PopArg("lock-internal-files") == "true"
			gotArg = req.PopArg("store-id")
		},
	})

	err := app.RunCLI(t.Context(), []string{"init", "-encryption", "none", "-lock-internal-files=false", ".default"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI error: %v", err)
	}
	if gotEncryption != "none" {
		t.Errorf("encryption = %q, want %q", gotEncryption, "none")
	}
	if gotLock {
		t.Errorf("lock-internal-files = true, want false")
	}
	if gotArg != ".default" {
		t.Errorf("store-id = %q, want %q", gotArg, ".default")
	}
}

func TestParseFlagsStopsAfterFirstPositional(t *testing.T) {
	params := []OldParam{
		{Name: "format", Type: String, Description: "Output format"},
	}

	vals := make(map[string]any)
	remaining, err := parseFlags([]string{"positional", "--format", "tap"}, params, vals)
	if err != nil {
		t.Fatalf("parseFlags error: %v", err)
	}
	// Once "positional" is seen (non-flag), everything after is positional.
	if _, set := vals["format"]; set {
		t.Error("--format should NOT be parsed as a flag after a positional arg")
	}
	if len(remaining) != 3 {
		t.Errorf("remaining = %v, want 3 items", remaining)
	}
}

func TestParseFlagsDoubleDashTerminatesFlags(t *testing.T) {
	params := []OldParam{
		{Name: "verbose", Type: Bool, Description: "Verbose"},
	}

	vals := make(map[string]any)
	remaining, err := parseFlags([]string{"--", "--verbose", "file.txt"}, params, vals)
	if err != nil {
		t.Fatalf("parseFlags error: %v", err)
	}
	if _, set := vals["verbose"]; set {
		t.Error("--verbose should NOT be parsed after --")
	}
	if len(remaining) != 2 || remaining[0] != "--verbose" || remaining[1] != "file.txt" {
		t.Errorf("remaining = %v, want [--verbose file.txt]", remaining)
	}
}

func TestRunCLIVariadicArg(t *testing.T) {
	app := NewUtility("test", "test app")

	var gotArgs []string
	app.AddCmd("sync", &captureParamCmd{
		params: []Param{
			StringArg{Name: "store-ids", Description: "Store IDs", Variadic: true},
		},
		onRun: func(req Request) {
			gotArgs = req.PopArgs()
		},
	})

	err := app.RunCLI(t.Context(), []string{"sync", ".default", ".sha256", ".backup"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI error: %v", err)
	}
	if len(gotArgs) != 3 {
		t.Fatalf("PopArgs() returned %d args, want 3: %v", len(gotArgs), gotArgs)
	}
	if gotArgs[0] != ".default" || gotArgs[1] != ".sha256" || gotArgs[2] != ".backup" {
		t.Errorf("PopArgs() = %v, want [.default .sha256 .backup]", gotArgs)
	}
}

func TestRunCLIVariadicArgWithFlags(t *testing.T) {
	app := NewUtility("test", "test app")

	var gotVerbose, gotArgs string
	app.AddCmd("sync", &captureParamCmd{
		params: []Param{
			BoolFlag{Name: "verbose", Description: "Verbose", Short: 'v'},
			StringArg{Name: "store-ids", Description: "Store IDs", Variadic: true},
		},
		onRun: func(req Request) {
			gotVerbose = req.PopArg("verbose")
			gotArgs = strings.Join(req.PopArgs(), ",")
		},
	})

	err := app.RunCLI(t.Context(), []string{"sync", "-v", ".default", ".sha256"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI error: %v", err)
	}
	if gotVerbose != "true" {
		t.Errorf("verbose = %q, want %q", gotVerbose, "true")
	}
	if gotArgs != ".default,.sha256" {
		t.Errorf("store-ids = %q, want %q", gotArgs, ".default,.sha256")
	}
}

func TestShortFlagNotInJSONSchema(t *testing.T) {
	cmd := &Command{
		Name: "status",
		OldParams: []OldParam{
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
