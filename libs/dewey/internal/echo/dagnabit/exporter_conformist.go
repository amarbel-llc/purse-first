package dagnabit

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	toml "github.com/BurntSushi/toml"
	"github.com/gobwas/glob"

	"code.linenisgreat.com/purse-first/libs/dewey/pkgs/xdg"
)

// ceilingEnvVar is the GIT_CEILING_DIRECTORIES-style env var that bounds
// findConformistConfig's upward walk: a colon-separated list of absolute
// directories the walk will not ascend into. Without it, a repo that has
// migrated to a Nix-generated conformist config (no conformist.toml on disk)
// would escalate past its own root and pick up a stray ancestor config (e.g.
// an eng-root ~/eng/conformist.toml) — purse-first#159. Mirrors madder's
// MADDER_CEILING_DIRECTORIES; resolved via dewey's xdg ceiling primitives.
var ceilingEnvVar = xdg.CeilingEnvVarName("dagnabit")

// conformistConfigEnvVar names the facade config dagnabit formats its generated
// files with (`conformist --config-file`). Generated files belong to the
// generator: a repo's own conformist config excludes them, and dagnabit formats
// them with this dedicated, formatters-only config instead (see
// validateFacadeConfig for the contract, and purse-first's
// lib.conformistModules.dagnabit-facade for a module that builds one). conformist's
// own --config-file has no env var and otherwise searches upward for a
// conformist.toml, which doesn't exist on disk for a Nix-generated config
// (purse-first#159).
const conformistConfigEnvVar = "DAGNABIT_CONFORMIST_CONFIG"

// conformistConfigNames are the config filenames that indicate a conformist
// setup, searched in order. (The legacy treefmt fallback — treefmt.toml,
// .treefmt.toml, treefmt.nix, and the `nix fmt` path — was removed once the
// last treefmt-configured consumer repos migrated to conformist; eng#246.)
var conformistConfigNames = []string{
	"conformist.toml",
	".conformist.toml",
}

// conformistTreeRootEnvVars are the ambient env vars that set conformist's tree
// root (the CONFORMIST_ prefix and its legacy TREELINT_ fallback). They are
// stripped from the conformist child: dagnabit decides the tree root itself,
// and a *_FILE/*_CMD value alongside an explicit --tree-root is a hard error in
// conformist ("at most one of tree-root, tree-root-cmd or tree-root-file").
var conformistTreeRootEnvVars = []string{
	"CONFORMIST_TREE_ROOT",
	"CONFORMIST_TREE_ROOT_FILE",
	"CONFORMIST_TREE_ROOT_CMD",
	"TREELINT_TREE_ROOT",
	"TREELINT_TREE_ROOT_FILE",
	"TREELINT_TREE_ROOT_CMD",
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

// FormatOutput formats the generated facades with conformist. No-op when
// DryRun is set, when the output dir does not exist, or when no config is
// found. See formatGeneratedOutput.
func (exporter *Exporter) FormatOutput() error {
	if exporter.DryRun {
		return nil
	}

	return formatGeneratedOutput(
		exporter.Dir,
		exporter.outputRoot(),
		filepath.Join(exporter.outputRoot(), exporter.outputDir()),
	)
}

// formatGeneratedOutput runs conformist over outputPath, anchoring the tree root
// at treeRoot: the module root for a real run, or the in-tree temp root that
// `--check` renders into, so generated files are always seen as
// `<outputDir>/...` relative paths.
//
// Resolution order:
//  0. If DAGNABIT_CONFORMIST_CONFIG names a config, validate it as a facade
//     config (validateFacadeConfig) and run conformist with it.
//  1. Otherwise search upward from moduleDir (bounded by
//     DAGNABIT_CEILING_DIRECTORIES) for a conformist.toml/.conformist.toml and
//     run conformist with it, warning that this discovery path is deprecated:
//     a repo's own config excludes the generated files, so it is the wrong
//     config for this pass.
//  2. A found config with no `conformist` on PATH is an error rather than a
//     silent skip, so unformatted output never reads as phantom drift.
func formatGeneratedOutput(moduleDir, treeRoot, outputPath string) error {
	if outputExists, err := outputDirExists(outputPath); err != nil {
		return err
	} else if !outputExists {
		return nil
	}

	if configFile := os.Getenv(conformistConfigEnvVar); configFile != "" {
		if err := validateFacadeConfig(configFile, treeRoot, outputPath); err != nil {
			return err
		}
		return runConformist(treeRoot, outputPath, configFile)
	}

	configDir, configName, ok := findConformistConfig(moduleDir)
	if !ok {
		return nil
	}

	if _, err := exec.LookPath("conformist"); err != nil {
		return fmt.Errorf(
			"formatter config %s found at %s, but `conformist` is not on PATH;"+
				" refusing to skip formatting — run inside the dev shell so"+
				" `conformist` is available",
			configName, configDir,
		)
	}

	fmt.Fprintf(
		os.Stderr,
		"dagnabit: warning: formatting generated files with discovered %s;"+
			" this is deprecated — set %s to a facade config (see dagnabit(1))\n",
		filepath.Join(configDir, configName), conformistConfigEnvVar,
	)

	return runConformist(treeRoot, outputPath, filepath.Join(configDir, configName))
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

// facadeConformistConfig is the subset of a conformist config that
// validateFacadeConfig inspects. Key names mirror conformist's config/config.go
// (Config, Formatter, Linter); a key renamed there must be renamed here, or the
// check silently stops seeing it.
type facadeConformistConfig struct {
	Excludes []string `toml:"excludes"`
	Global   struct {
		Excludes []string `toml:"excludes"`
	} `toml:"global"`
	SkipGenerated bool                                 `toml:"skip-generated"`
	TreeRoot      string                               `toml:"tree-root"`
	TreeRootCmd   string                               `toml:"tree-root-cmd"`
	TreeRootFile  string                               `toml:"tree-root-file"`
	Formatter     map[string]facadeConformistFormatter `toml:"formatter"`
	Linter        map[string]toml.Primitive            `toml:"linter"`
}

type facadeConformistFormatter struct {
	Excludes   []string `toml:"excludes"`
	WorkingDir string   `toml:"working-dir"`
}

// validateFacadeConfig enforces the DAGNABIT_CONFORMIST_CONFIG contract. The
// facade config formats only the files dagnabit generates, with the tree root
// at treeRoot, so it must not carry anything written for a repo's own
// whole-tree runs:
//   - no [linter.*]: conformist runs whole-tree linters at the tree root even
//     when their includes are excluded (purse-first#195: codegen-repair);
//   - no tree-root, tree-root-cmd or tree-root-file: they collide with
//     dagnabit's --tree-root;
//   - no skip-generated (conformist#133): every file dagnabit writes carries
//     the "Code generated … DO NOT EDIT." stamp, so it would format nothing;
//   - no formatter working-dir: it resolves against dagnabit's tree root, so a
//     repo-root-relative value such as "go" descends a second time;
//   - no exclude (global or per-formatter) matching a generated file: it would
//     silently leave that file unformatted. Excludes are matched with the same
//     glob library and tree-root-relative paths conformist uses, so defaults
//     such as "*.lock" or "vendor/*" pass.
func validateFacadeConfig(configFile, treeRoot, outputPath string) error {
	var cfg facadeConformistConfig
	if _, err := toml.DecodeFile(configFile, &cfg); err != nil {
		return fmt.Errorf("reading %s %s: %w", conformistConfigEnvVar, configFile, err)
	}

	var problems []string

	for _, name := range slices.Sorted(maps.Keys(cfg.Linter)) {
		problems = append(problems, fmt.Sprintf("declares linter %q", name))
	}

	if cfg.SkipGenerated {
		problems = append(problems, "sets skip-generated (it withholds every file dagnabit generates)")
	}

	for key, value := range map[string]string{
		"tree-root":      cfg.TreeRoot,
		"tree-root-cmd":  cfg.TreeRootCmd,
		"tree-root-file": cfg.TreeRootFile,
	} {
		if value != "" {
			problems = append(problems, fmt.Sprintf("sets %s", key))
		}
	}

	generated, err := generatedRelPaths(treeRoot, outputPath)
	if err != nil {
		return err
	}

	excludeProblems, err := excludesMatchingGenerated(
		"global", slices.Concat(cfg.Excludes, cfg.Global.Excludes), generated,
	)
	if err != nil {
		return err
	}
	problems = append(problems, excludeProblems...)

	for _, name := range slices.Sorted(maps.Keys(cfg.Formatter)) {
		formatter := cfg.Formatter[name]
		if formatter.WorkingDir != "" {
			problems = append(problems, fmt.Sprintf(
				"formatter %q sets working-dir %q", name, formatter.WorkingDir,
			))
		}

		excludeProblems, err := excludesMatchingGenerated(
			fmt.Sprintf("formatter %q", name), formatter.Excludes, generated,
		)
		if err != nil {
			return err
		}
		problems = append(problems, excludeProblems...)
	}

	if len(problems) == 0 {
		return nil
	}

	slices.Sort(problems)
	return fmt.Errorf(
		"%s=%s is not a dagnabit facade config (formatters only; see dagnabit(1),"+
			" or build one from purse-first's lib.conformistModules.dagnabit-facade):\n  - %s",
		conformistConfigEnvVar, configFile, strings.Join(problems, "\n  - "),
	)
}

// generatedRelPaths lists every regular file under outputPath as a
// slash-separated path relative to treeRoot — the form conformist matches its
// include/exclude globs against.
func generatedRelPaths(treeRoot, outputPath string) ([]string, error) {
	var paths []string

	err := filepath.WalkDir(outputPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return nil
		}

		rel, err := filepath.Rel(treeRoot, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing generated files under %s: %w", outputPath, err)
	}

	return paths, nil
}

// excludesMatchingGenerated reports each exclude pattern that matches one of
// the generated paths, compiled with gobwas/glob and no separators, exactly as
// conformist compiles them (so `*` and `**` cross `/`; conformist's
// format/glob.go). If conformist changes glob library, this must follow.
func excludesMatchingGenerated(scope string, excludes, generated []string) ([]string, error) {
	var problems []string

	for _, pattern := range excludes {
		compiled, err := glob.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("compiling %s exclude %q: %w", scope, pattern, err)
		}

		if index := slices.IndexFunc(generated, compiled.Match); index >= 0 {
			problems = append(problems, fmt.Sprintf(
				"%s exclude %q matches generated file %s", scope, pattern, generated[index],
			))
		}
	}

	return problems, nil
}

// runConformist formats outputPath with the `conformist` binary, run from
// treeRoot with configFile as its --config-file.
//
// conformist defaults to a git walk anchored at the worktree root, which skips
// untracked paths — including freshly generated facades and the temp dir used
// by `export --check`. `--walk filesystem` formats every generated file
// regardless of git status; the positional outputPath narrows the walk to them.
//
// The exception to passing --tree-root is the Nix-generated conformist
// *wrapper* (purse-first#162): it execs `conformist --config-file=<store>
// --tree-root-file=<projectRootFile> "$@"`, baking a tree root that conformist
// treats as mutually exclusive with --tree-root. When the resolved conformist
// already bakes a tree root, the wrapper's --tree-root-file stands.
func runConformist(treeRoot, outputPath, configFile string) error {
	conformistPath, err := exec.LookPath("conformist")
	if err != nil {
		return fmt.Errorf("conformist: %w", err)
	}

	var args []string
	if !conformistBakesTreeRoot(conformistPath) {
		args = append(args, "--tree-root", treeRoot)
	}
	args = append(args, "--walk", "filesystem")
	if configFile != "" {
		args = append(args, "--config-file", configFile)
	}
	args = append(args, outputPath)

	cmd := exec.Command(conformistPath, args...)
	cmd.Dir = treeRoot
	cmd.Env = environWithoutTreeRoot(os.Environ())
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("conformist: %w", err)
	}
	return nil
}

// environWithoutTreeRoot drops conformistTreeRootEnvVars from environ.
func environWithoutTreeRoot(environ []string) []string {
	return slices.DeleteFunc(slices.Clone(environ), func(entry string) bool {
		name, _, _ := strings.Cut(entry, "=")
		return slices.Contains(conformistTreeRootEnvVars, name)
	})
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
// (see conformist's nix/module-options.nix build.wrapper).
//
// Only a script (first bytes `#!`) is a wrapper candidate. The raw conformist
// binary is an ELF/Mach-O that DOES carry the literal flag names in its own
// help and warning strings ("pass --tree-root to override"), so scanning a
// binary's bytes misclassified it as the wrapper, dropped dagnabit's
// --tree-root, and let conformist anchor at the module root — where the
// whole-tree codegen-repair lane then failed in the nix sandbox
// (purse-first#195). A read error, a non-script, or an absent flag falls back
// to the raw-binary assumption (append --tree-root).
func conformistBakesTreeRoot(conformistPath string) bool {
	contents, err := os.ReadFile(conformistPath)
	if err != nil {
		return false
	}
	body := string(contents)
	if !strings.HasPrefix(body, "#!") {
		return false
	}
	for _, flag := range treeRootBakingFlags {
		if strings.Contains(body, flag) {
			return true
		}
	}
	return false
}
