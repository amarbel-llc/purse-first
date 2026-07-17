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
// findConformistConfig's upward walk: a colon-separated list of absolute
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

// conformistConfigNames are the config filenames that indicate a conformist
// setup, searched in order. (The legacy treefmt fallback — treefmt.toml,
// .treefmt.toml, treefmt.nix, and the `nix fmt` path — was removed once the
// last treefmt-configured consumer repos migrated to conformist; eng#246.)
var conformistConfigNames = []string{
	"conformist.toml",
	".conformist.toml",
}

// findConformistConfig walks up from start looking for a conformist config
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
func findConformistConfig(start string) (dir, name string, ok bool) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", "", false
	}

	ceilings := xdg.ParseCeilingDirectories(os.Getenv(ceilingEnvVar))

	for {
		for _, candidate := range conformistConfigNames {
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

// FormatOutput runs conformist on the output directory if a conformist
// configuration is present in the module's directory tree. No-op when no
// config is found or when DryRun is set.
//
// Resolution order:
//  0. If DAGNABIT_CONFORMIST_CONFIG names an explicit conformist config, run
//     `conformist --config-file <that>` — skipping the upward config search
//     entirely. This is how a Nix-generated conformist config (no
//     conformist.toml on disk) is honored (purse-first#159).
//  1. Otherwise search upward (bounded by DAGNABIT_CEILING_DIRECTORIES) for a
//     conformist.toml/.conformist.toml and run `conformist` on the output
//     dir, if that binary is on PATH.
//  2. Otherwise emit an error: a config-present tree with no conformist on
//     PATH must fail loud rather than silently skip formatting.
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

	configDir, configName, ok := findConformistConfig(exporter.Dir)
	if !ok {
		return nil
	}

	if _, err := exec.LookPath("conformist"); err != nil {
		// Fail loud rather than silently emitting unformatted facades: a
		// missing formatter in a config-present tree means generated output
		// would diff against the committed (formatted) facades for the wrong
		// reason. Callers must run inside an environment where conformist is
		// on PATH (e.g. the dev shell).
		return fmt.Errorf(
			"formatter config %s found at %s, but `conformist` is not on PATH;"+
				" refusing to skip formatting — run inside the dev shell so"+
				" `conformist` is available",
			configName, configDir,
		)
	}

	return runConformist(configDir, outputPath, "")
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
//
// The exception is the Nix-generated conformist *wrapper* (purse-first#162):
// it execs `conformist --config-file=<store> --tree-root-file=<projectRootFile>
// "$@"`, baking a tree root that conformist treats as mutually exclusive with
// our --tree-root ("if any flags in the group [tree-root tree-root-cmd
// tree-root-file] are set none of the others can be"). When the resolved
// conformist already bakes a tree root, we omit --tree-root and let the
// wrapper's --tree-root-file stand; --walk filesystem and the positional
// outputPath still scope the walk to the generated files.
func runConformist(workDir, outputPath, configFile string) error {
	conformistPath, err := exec.LookPath("conformist")
	if err != nil {
		return fmt.Errorf("conformist: %w", err)
	}

	var args []string
	if !conformistBakesTreeRoot(conformistPath) {
		args = append(args, "--tree-root", outputPath)
	}
	args = append(args, "--walk", "filesystem")
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

// treeRootBakingFlags are the conformist tree-root flags whose presence in the
// resolved conformist invocation means dagnabit must NOT add its own
// --tree-root: conformist rejects setting more than one of the
// [tree-root tree-root-cmd tree-root-file] group.
var treeRootBakingFlags = []string{
	"--tree-root-file",
	"--tree-root-cmd",
	"--tree-root",
}

// conformistBakesTreeRoot reports whether the resolved conformist is the
// Nix-generated wrapper, which execs the raw binary with a tree-root flag
// already baked in (purse-first#162). The wrapper is a `writeShellScriptBin`
// shell script whose body contains a literal --tree-root-file=<projectRootFile>
// (see conformist's nix/module-options.nix build.wrapper); the raw binary is an
// ELF/Mach-O whose bytes won't carry the literal flag. Reading the resolved
// path and scanning for a tree-root flag distinguishes the two without a new
// env signal to plumb. A read error or absent flag falls back to the raw-binary
// assumption (append --tree-root), preserving the pre-#162 behavior.
func conformistBakesTreeRoot(conformistPath string) bool {
	contents, err := os.ReadFile(conformistPath)
	if err != nil {
		return false
	}
	body := string(contents)
	for _, flag := range treeRootBakingFlags {
		if strings.Contains(body, flag) {
			return true
		}
	}
	return false
}
