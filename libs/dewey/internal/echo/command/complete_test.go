package command

import (
	"context"
	"encoding/json"
	"testing"
)

func TestCompleteDispatchesToCompleter(t *testing.T) {
	app := NewUtility("myapp", "My app")
	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		OldParams: []OldParam{
			{Name: "repo_path", Type: String, Description: "Path to repo"},
			{
				Name:        "branch",
				Type:        String,
				Description: "Branch name",
				Completer: func() map[string]string {
					return map[string]string{
						"main":    "default branch",
						"develop": "development branch",
					}
				},
			},
		},
	})

	// Call __complete for a param with a Completer
	err := app.RunCLI(context.Background(), []string{
		"__complete", "--command", "status", "--param", "branch",
	}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI __complete: %v", err)
	}
}

func TestCompleteUnknownCommandReturnsNoError(t *testing.T) {
	app := NewUtility("myapp", "My app")

	err := app.RunCLI(context.Background(), []string{
		"__complete", "--command", "nonexistent", "--param", "foo",
	}, StubPrompter{})
	if err != nil {
		t.Fatalf("expected no error for unknown command, got: %v", err)
	}
}

func TestCompleteParamWithoutCompleterReturnsNoError(t *testing.T) {
	app := NewUtility("myapp", "My app")
	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		OldParams: []OldParam{
			{Name: "repo_path", Type: String, Description: "Path to repo"},
		},
	})

	err := app.RunCLI(context.Background(), []string{
		"__complete", "--command", "status", "--param", "repo_path",
	}, StubPrompter{})
	if err != nil {
		t.Fatalf("expected no error for param without completer, got: %v", err)
	}
}

func TestCompleteIsHidden(t *testing.T) {
	app := NewUtility("myapp", "My app")

	for name, cmd := range app.AllCommands() {
		if name == "__complete" {
			if !cmd.Hidden {
				t.Error("__complete command should be hidden")
			}
			return
		}
	}
	t.Error("__complete command not found")
}

func TestCompleteHandlerDirectly(t *testing.T) {
	app := NewUtility("myapp", "My app")
	app.AddCommand(&Command{
		Name:        "deploy",
		Description: Description{Short: "Deploy app"},
		OldParams: []OldParam{
			{
				Name:        "env",
				Type:        String,
				Description: "Target environment",
				Completer: func() map[string]string {
					return map[string]string{
						"staging":    "staging environment",
						"production": "production environment",
					}
				},
			},
		},
	})

	cmd, ok := app.GetCommand("__complete")
	if !ok {
		t.Fatal("__complete command not registered")
	}

	argsJSON, _ := json.Marshal(map[string]string{
		"command": "deploy",
		"param":   "env",
	})
	err := cmd.RunCLI(context.Background(), argsJSON)
	if err != nil {
		t.Fatalf("__complete handler: %v", err)
	}
}
