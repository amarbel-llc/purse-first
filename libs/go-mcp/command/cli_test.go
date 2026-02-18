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
