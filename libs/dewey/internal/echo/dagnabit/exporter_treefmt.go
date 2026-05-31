package dagnabit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// treefmtConfigNames are the config filenames that indicate a treelint or
// treefmt setup, searched in order. treelint (the treefmt successor) is
// preferred; plain treefmt and treefmt-nix remain as fallbacks.
var treefmtConfigNames = []string{
	"treelint.toml",
	".treelint.toml",
	"treefmt.toml",
	".treefmt.toml",
	"treefmt.nix",
}

// findTreefmtConfig walks up from start looking for a treefmt config
// file. Returns the directory containing the config, the config
// filename, and ok=true on success. Walking stops at the filesystem
// root.
func findTreefmtConfig(start string) (dir, name string, ok bool) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", "", false
	}

	for {
		for _, candidate := range treefmtConfigNames {
			if _, err := os.Stat(filepath.Join(abs, candidate)); err == nil {
				return abs, candidate, true
			}
		}

		parent := filepath.Dir(abs)
		if parent == abs {
			return "", "", false
		}
		abs = parent
	}
}

// FormatOutput runs the project's formatter on the output directory if a
// treelint or treefmt configuration is present in the module's directory
// tree. No-op when no config is found or when DryRun is set.
//
// Resolution order:
//  1. `<formatter> <output-dir>` where <formatter> is `treelint` for a
//     treelint.toml/.treelint.toml config and `treefmt` for a
//     treefmt.toml/.treefmt.toml/treefmt.nix config, if that binary is on
//     PATH.
//  2. `nix fmt -- <output-dir>` if config is `treefmt.nix` and `nix` is on
//     PATH.
//  3. Otherwise emit a warning to stderr and skip.
//
// Invocation runs with cwd set to the directory containing the config so
// the formatter resolves project-relative paths the same way the user's
// own invocations do.
func (exporter *Exporter) FormatOutput() error {
	if exporter.DryRun {
		return nil
	}

	configDir, configName, ok := findTreefmtConfig(exporter.Dir)
	if !ok {
		return nil
	}

	outputPath := filepath.Join(exporter.Dir, exporter.outputDir())
	if _, err := os.Stat(outputPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat output dir: %w", err)
	}

	formatter := "treefmt"
	if strings.Contains(configName, "treelint") {
		formatter = "treelint"
	}

	if formatterPath, err := exec.LookPath(formatter); err == nil {
		cmd := exec.Command(formatterPath, outputPath)
		cmd.Dir = configDir
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s: %w", formatter, err)
		}
		return nil
	}

	if configName == "treefmt.nix" {
		if nixPath, err := exec.LookPath("nix"); err == nil {
			cmd := exec.Command(nixPath, "fmt", "--", outputPath)
			cmd.Dir = configDir
			cmd.Stdout = os.Stderr
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				return fmt.Errorf("nix fmt: %w", err)
			}
			return nil
		}
	}

	fmt.Fprintf(
		os.Stderr,
		"warning: formatter config %s found at %s, but neither `%s` nor `nix fmt` is available; skipping format pass\n",
		configName, configDir, formatter,
	)
	return nil
}
