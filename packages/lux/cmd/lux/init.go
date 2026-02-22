package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/amarbel-llc/lux/internal/config"
	"github.com/amarbel-llc/lux/internal/config/filetype"
)

func runInit(useDefault, force bool) error {
	configDir := config.ConfigDir()
	filetypeDir := filetype.GlobalDir()

	if err := os.MkdirAll(filetypeDir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", filetypeDir, err)
	}
	fmt.Printf("created %s/\n", filetypeDir)

	var files map[string]string
	if useDefault {
		files = defaultConfigFiles(configDir, filetypeDir)
	} else {
		files = emptyConfigFiles(configDir)
	}

	for path, content := range files {
		if err := writeConfigFile(path, content, force); err != nil {
			return err
		}
	}

	return nil
}

func writeConfigFile(path, content string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("skipped %s (already exists, use --force to overwrite)\n", path)
			return nil
		}
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	fmt.Printf("wrote %s\n", path)
	return nil
}

func emptyConfigFiles(configDir string) map[string]string {
	return map[string]string{
		filepath.Join(configDir, "lsps.toml"):       "",
		filepath.Join(configDir, "formatters.toml"): "",
	}
}

func defaultConfigFiles(configDir, filetypeDir string) map[string]string {
	return map[string]string{
		filepath.Join(configDir, "lsps.toml"):         defaultLSPsConfig,
		filepath.Join(configDir, "formatters.toml"):   defaultFormattersConfig,
		filepath.Join(filetypeDir, "go.toml"):         defaultFiletypeGo,
		filepath.Join(filetypeDir, "python.toml"):     defaultFiletypePython,
		filepath.Join(filetypeDir, "javascript.toml"): defaultFiletypeJavascript,
		filepath.Join(filetypeDir, "typescript.toml"): defaultFiletypeTypescript,
		filepath.Join(filetypeDir, "rust.toml"):       defaultFiletypeRust,
		filepath.Join(filetypeDir, "lua.toml"):        defaultFiletypeLua,
		filepath.Join(filetypeDir, "nix.toml"):        defaultFiletypeNix,
		filepath.Join(filetypeDir, "shell.toml"):      defaultFiletypeShell,
	}
}

const defaultLSPsConfig = `[[lsp]]
name = "gopls"
flake = "nixpkgs#gopls"

[[lsp]]
name = "pyright"
flake = "nixpkgs#pyright"

[[lsp]]
name = "typescript-language-server"
flake = "nixpkgs#nodePackages.typescript-language-server"

[[lsp]]
name = "rust-analyzer"
flake = "nixpkgs#rust-analyzer"

[[lsp]]
name = "lua-language-server"
flake = "nixpkgs#lua-language-server"

[[lsp]]
name = "nil"
flake = "nixpkgs#nil"

[[lsp]]
name = "bash-language-server"
flake = "nixpkgs#nodePackages.bash-language-server"
`

const defaultFormattersConfig = `[[formatter]]
name = "golines"
flake = "nixpkgs#golines"

[[formatter]]
name = "isort"
flake = "nixpkgs#isort"

[[formatter]]
name = "black"
flake = "nixpkgs#black"

[[formatter]]
name = "prettierd"
flake = "nixpkgs#prettierd"

[[formatter]]
name = "prettier"
flake = "nixpkgs#nodePackages.prettier"

[[formatter]]
name = "nixfmt"
flake = "nixpkgs#nixfmt-rfc-style"

[[formatter]]
name = "rustfmt"
flake = "nixpkgs#rustfmt"

[[formatter]]
name = "stylua"
flake = "nixpkgs#stylua"

[[formatter]]
name = "shfmt"
flake = "nixpkgs#shfmt"
args = ["-s", "-i=2"]
`

const defaultFiletypeGo = `extensions = ["go"]
language_ids = ["go"]
lsp = "gopls"
formatters = ["golines"]
`

const defaultFiletypePython = `extensions = ["py"]
language_ids = ["python"]
lsp = "pyright"
formatters = ["isort", "black"]
formatter_mode = "chain"
`

const defaultFiletypeJavascript = `extensions = ["js", "jsx"]
language_ids = ["javascript", "javascriptreact"]
lsp = "typescript-language-server"
formatters = ["prettierd", "prettier"]
formatter_mode = "fallback"
`

const defaultFiletypeTypescript = `extensions = ["ts", "tsx"]
language_ids = ["typescript", "typescriptreact"]
lsp = "typescript-language-server"
formatters = ["prettierd", "prettier"]
formatter_mode = "fallback"
`

const defaultFiletypeRust = `extensions = ["rs"]
language_ids = ["rust"]
lsp = "rust-analyzer"
formatters = ["rustfmt"]
lsp_format = "fallback"
`

const defaultFiletypeLua = `extensions = ["lua"]
language_ids = ["lua"]
lsp = "lua-language-server"
formatters = ["stylua"]
`

const defaultFiletypeNix = `extensions = ["nix"]
language_ids = ["nix"]
lsp = "nil"
formatters = ["nixfmt"]
`

const defaultFiletypeShell = `extensions = ["sh", "bash"]
language_ids = ["shellscript"]
lsp = "bash-language-server"
formatters = ["shfmt"]
`
