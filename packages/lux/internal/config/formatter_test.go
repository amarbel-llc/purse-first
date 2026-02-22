package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatterValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     FormatterConfig
		wantErr bool
	}{
		{
			name: "valid with flake",
			cfg: FormatterConfig{
				Formatters: []Formatter{
					{Name: "gofumpt", Flake: "nixpkgs#gofumpt"},
				},
			},
		},
		{
			name: "valid with path",
			cfg: FormatterConfig{
				Formatters: []Formatter{
					{Name: "prettier", Path: "/usr/bin/prettier"},
				},
			},
		},
		{
			name: "missing name",
			cfg: FormatterConfig{
				Formatters: []Formatter{
					{Flake: "nixpkgs#gofumpt"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing flake and path",
			cfg: FormatterConfig{
				Formatters: []Formatter{
					{Name: "gofumpt"},
				},
			},
			wantErr: true,
		},
		{
			name: "both flake and path",
			cfg: FormatterConfig{
				Formatters: []Formatter{
					{Name: "gofumpt", Flake: "nixpkgs#gofumpt", Path: "/usr/bin/gofumpt"},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate names",
			cfg: FormatterConfig{
				Formatters: []Formatter{
					{Name: "gofumpt", Flake: "nixpkgs#gofumpt"},
					{Name: "gofumpt", Flake: "nixpkgs#gofumpt2"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid mode",
			cfg: FormatterConfig{
				Formatters: []Formatter{
					{Name: "gofumpt", Flake: "nixpkgs#gofumpt", Mode: "invalid"},
				},
			},
			wantErr: true,
		},
		{
			name: "valid stdin mode",
			cfg: FormatterConfig{
				Formatters: []Formatter{
					{Name: "gofumpt", Flake: "nixpkgs#gofumpt", Mode: FormatterModeStdin},
				},
			},
		},
		{
			name: "valid filepath mode",
			cfg: FormatterConfig{
				Formatters: []Formatter{
					{Name: "gofumpt", Flake: "nixpkgs#gofumpt", Mode: FormatterModeFilepath},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMergeFormatters(t *testing.T) {
	global := &FormatterConfig{
		Formatters: []Formatter{
			{Name: "gofumpt", Flake: "nixpkgs#gofumpt"},
			{Name: "prettier", Flake: "nixpkgs#prettier"},
			{Name: "nixfmt", Flake: "nixpkgs#nixfmt"},
		},
	}

	t.Run("local overrides global", func(t *testing.T) {
		local := &FormatterConfig{
			Formatters: []Formatter{
				{Name: "gofumpt", Path: "/custom/gofumpt"},
			},
		}

		merged := MergeFormatters(global, local)
		if len(merged.Formatters) != 3 {
			t.Fatalf("expected 3 formatters, got %d", len(merged.Formatters))
		}

		var gofumpt *Formatter
		for i := range merged.Formatters {
			if merged.Formatters[i].Name == "gofumpt" {
				gofumpt = &merged.Formatters[i]
			}
		}
		if gofumpt == nil {
			t.Fatal("expected gofumpt in merged config")
		}
		if gofumpt.Path != "/custom/gofumpt" {
			t.Errorf("expected custom path, got %q", gofumpt.Path)
		}
	})

	t.Run("local disables global", func(t *testing.T) {
		local := &FormatterConfig{
			Formatters: []Formatter{
				{Name: "prettier", Disabled: true},
			},
		}

		merged := MergeFormatters(global, local)
		if len(merged.Formatters) != 2 {
			t.Fatalf("expected 2 formatters, got %d", len(merged.Formatters))
		}

		for _, f := range merged.Formatters {
			if f.Name == "prettier" {
				t.Error("prettier should have been disabled")
			}
		}
	})

	t.Run("local adds new formatter", func(t *testing.T) {
		local := &FormatterConfig{
			Formatters: []Formatter{
				{Name: "shfmt", Path: "/usr/bin/shfmt"},
			},
		}

		merged := MergeFormatters(global, local)
		if len(merged.Formatters) != 4 {
			t.Fatalf("expected 4 formatters, got %d", len(merged.Formatters))
		}
	})

	t.Run("empty local preserves global", func(t *testing.T) {
		local := &FormatterConfig{}
		merged := MergeFormatters(global, local)
		if len(merged.Formatters) != 3 {
			t.Fatalf("expected 3 formatters, got %d", len(merged.Formatters))
		}
	})
}

func TestEffectiveMode(t *testing.T) {
	t.Run("default is stdin", func(t *testing.T) {
		f := Formatter{}
		if f.EffectiveMode() != FormatterModeStdin {
			t.Errorf("expected stdin, got %s", f.EffectiveMode())
		}
	})

	t.Run("explicit stdin", func(t *testing.T) {
		f := Formatter{Mode: FormatterModeStdin}
		if f.EffectiveMode() != FormatterModeStdin {
			t.Errorf("expected stdin, got %s", f.EffectiveMode())
		}
	})

	t.Run("explicit filepath", func(t *testing.T) {
		f := Formatter{Mode: FormatterModeFilepath}
		if f.EffectiveMode() != FormatterModeFilepath {
			t.Errorf("expected filepath, got %s", f.EffectiveMode())
		}
	})
}

func TestExpandEnvVars(t *testing.T) {
	os.Setenv("LUX_TEST_HOME", "/test/home")
	defer os.Unsetenv("LUX_TEST_HOME")

	result := ExpandEnvVars("$LUX_TEST_HOME/bin/formatter")
	expected := "/test/home/bin/formatter"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestLoadFormatterFile(t *testing.T) {
	t.Run("missing file returns empty config", func(t *testing.T) {
		cfg, err := loadFormatterFile("/nonexistent/path/formatters.toml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.Formatters) != 0 {
			t.Errorf("expected empty formatters, got %d", len(cfg.Formatters))
		}
	})

	t.Run("valid toml file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "formatters.toml")

		content := `
[[formatter]]
name = "gofumpt"
flake = "nixpkgs#gofumpt"
extensions = ["go"]
mode = "stdin"

[[formatter]]
name = "prettier"
path = "/usr/bin/prettier"
extensions = ["js", "ts"]
args = ["--stdin-filepath", "{file}"]
`
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("writing test file: %v", err)
		}

		cfg, err := loadFormatterFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(cfg.Formatters) != 2 {
			t.Fatalf("expected 2 formatters, got %d", len(cfg.Formatters))
		}

		if cfg.Formatters[0].Name != "gofumpt" {
			t.Errorf("expected gofumpt, got %s", cfg.Formatters[0].Name)
		}
		if cfg.Formatters[0].Flake != "nixpkgs#gofumpt" {
			t.Errorf("expected nixpkgs#gofumpt, got %s", cfg.Formatters[0].Flake)
		}
		if cfg.Formatters[0].Mode != FormatterModeStdin {
			t.Errorf("expected stdin mode, got %s", cfg.Formatters[0].Mode)
		}

		if cfg.Formatters[1].Name != "prettier" {
			t.Errorf("expected prettier, got %s", cfg.Formatters[1].Name)
		}
		if len(cfg.Formatters[1].Args) != 2 {
			t.Errorf("expected 2 args, got %d", len(cfg.Formatters[1].Args))
		}
	})

	t.Run("toml with env and disabled", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "formatters.toml")

		content := `
[[formatter]]
name = "prettier"
path = "$HOME/bin/prettier"
extensions = ["js"]
disabled = true

[formatter.env]
NODE_ENV = "production"
`
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("writing test file: %v", err)
		}

		cfg, err := loadFormatterFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(cfg.Formatters) != 1 {
			t.Fatalf("expected 1 formatter, got %d", len(cfg.Formatters))
		}

		f := cfg.Formatters[0]
		if !f.Disabled {
			t.Error("expected disabled to be true")
		}
		if f.Env["NODE_ENV"] != "production" {
			t.Errorf("expected NODE_ENV=production, got %q", f.Env["NODE_ENV"])
		}
	})
}
