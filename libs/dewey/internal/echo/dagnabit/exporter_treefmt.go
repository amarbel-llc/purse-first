package dagnabit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/amarbel-llc/purse-first/libs/dewey/pkgs/xdg"
)

// ceilingEnvVar is the GIT_CEILING_DIRECTORIES-style env var that bounds
// findTreefmtConfig's upward walk: a colon-separated list of absolute
// directories the walk will not ascend into. Without it, a repo that has
// migrated to a Nix-generated conformist config (no conformist.toml on disk)
// would escalate past its own root and pick up a stray ancestor config (e.g.
// an eng-root ~/eng/conformist.toml) — purse-first#159. Mirrors madder's
// MADDER_CEILING_DIRECTORIES; resolved via dewey's xdg ceiling primitives.
var ceilingEnvVar = xdg.CeilingEnvVarName("dagnabit")

// conformistConfigEnvVar names an explicit conformist config file for
// FormatOutput to pass via `conformist --config-file`. This is the escape
// hatch for a Nix-generated conformist config (purse-first#159): conformist's
// own --config-file has no env var and otherwise searches upward for a
// conformist.toml, which doesn't exist on disk for an evalModule-driven setup.
// A consumer (e.g. the lint-dewey_pkgs_drift recipe) generates the config —
// `conformist.lib.evalModule ... .config.build.configFile` — and points
// dagnabit at the resulting store path. When set, it short-circuits the upward
// config search entirely: the formatter is conformist and the config is known.
const conformistConfigEnvVar = "DAGNABIT_CONFORMIST_CONFIG"

// treefmtConfigNames are the config filenames that indicate a conformist or
// treefmt setup, searched in order. conformist (the treefmt successor) is
// preferred; plain treefmt and treefmt-nix remain as fallbacks.
var treefmtConfigNames = []string{
	"conformist.toml",
	".conformist.toml",
	"treefmt.toml",
	".treefmt.toml",
	"treefmt.nix",
}

// findTreefmtConfig walks up from start looking for a treefmt config
// file. Returns the directory containing the config, the config
// filename, and ok=true on success. Walking stops at the filesystem
// root, or — when DAGNABIT_CEILING_DIRECTORIES is set — before ascending
// into a ceiling directory.
//
// The ceiling bounds escalation the way git's GIT_CEILING_DIRECTORIES does:
// a ceiling entry is the last directory NOT searched, so the walk still
// checks every directory at and below the ceiling (including start) but
// never the ceiling itself or anything above it. This is what keeps a
// repo with a Nix-generated conformist config (no conformist.toml on disk)
// from picking up a stray ancestor config (purse-first#159).
func findTreefmtConfig(start string) (dir, name string, ok bool) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", "", false
	}

	ceilings := xdg.ParseCeilingDirectories(os.Getenv(ceilingEnvVar))

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
		// Refuse to ascend into (or above) a ceiling directory: the ceiling
		// is the last dir not searched, matching GIT_CEILING_DIRECTORIES.
		if len(ceilings) > 0 && xdg.IsAtOrAboveCeiling(parent, ceilings) {
			return "", "", false
		}
		abs = parent
	}
}

// FormatOutput runs the project's formatter on the output directory if a
// conformist or treefmt configuration is present in the module's directory
// tree. No-op when no config is found or when DryRun is set.
//
// Resolution order:
//  0. If DAGNABIT_CONFORMIST_CONFIG names an explicit conformist config, run
//     `conformist --config-file <that>` — skipping the upward config search
//     entirely. This is how a Nix-generated conformist config (no
//     conformist.toml on disk) is honored (purse-first#159).
//  1. Otherwise search upward (bounded by DAGNABIT_CEILING_DIRECTORIES) for a
//     config and run `<formatter> <output-dir>` where <formatter> is
//     `conformist` for a conformist.toml/.conformist.toml config and `treefmt`
//     for a treefmt.toml/.treefmt.toml/treefmt.nix config, if that binary is on
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

	outputPath := filepath.Join(exporter.outputRoot(), exporter.outputDir())
	if outputExists, err := outputDirExists(outputPath); err != nil {
		return err
	} else if !outputExists {
		return nil
	}

	// Explicit Nix-generated config short-circuits discovery (purse-first#159).
	if configFile := os.Getenv(conformistConfigEnvVar); configFile != "" {
		return runConformist(exporter.Dir, outputPath, configFile)
	}

	configDir, configName, ok := findTreefmtConfig(exporter.Dir)
	if !ok {
		return nil
	}

	formatter := "treefmt"
	if strings.Contains(configName, "conformist") {
		formatter = "conformist"
	}

	if formatter == "conformist" {
		if _, err := exec.LookPath("conformist"); err == nil {
			return runConformist(configDir, outputPath, "")
		}
	} else if formatterPath, err := exec.LookPath(formatter); err == nil {
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

	// Fail loud rather than silently emitting unformatted facades: a missing
	// formatter in a config-present tree means generated output would diff
	// against the committed (formatted) facades for the wrong reason. Callers
	// must run inside an environment where the configured formatter is on PATH
	// (e.g. the dev shell).
	return fmt.Errorf(
		"formatter config %s found at %s, but `%s` is not on PATH"+
			" (and no `nix fmt` fallback); refusing to skip formatting"+
			" — run inside the dev shell so `%s` is available",
		configName, configDir, formatter, formatter,
	)
}

// outputDirExists reports whether the export output directory is present.
// A missing directory is a no-op for FormatOutput (ok=false, err=nil); any
// other stat error is surfaced.
func outputDirExists(outputPath string) (bool, error) {
	if _, err := os.Stat(outputPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat output dir: %w", err)
	}
	return true, nil
}

// runConformist formats outputPath with the `conformist` binary. configFile,
// when non-empty, is passed via --config-file to honor an explicit
// (e.g. Nix-generated) config instead of conformist's upward search.
//
// conformist defaults to a git walk anchored at the worktree root, which skips
// untracked paths — including freshly generated facades and the temp dir used
// by `export --check`. Anchoring the tree root at the output dir and walking
// the filesystem formats every generated file regardless of git status.
func runConformist(workDir, outputPath, configFile string) error {
	conformistPath, err := exec.LookPath("conformist")
	if err != nil {
		return fmt.Errorf("conformist: %w", err)
	}

	args := []string{"--tree-root", outputPath, "--walk", "filesystem"}
	if configFile != "" {
		args = append(args, "--config-file", configFile)
	}
	args = append(args, outputPath)

	cmd := exec.Command(conformistPath, args...)
	cmd.Dir = workDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("conformist: %w", err)
	}
	return nil
}
