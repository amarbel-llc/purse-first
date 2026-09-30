# dagnabit facade-format pass: facade ownership, facade config, tree root

Status: design, not implemented. Session purse-first/noble-rowan/pennywise, 2026-09-30.
Trigger: chrest `checks.dagnabit-codegen` fails after bumping purse-first to 7211f3a
(`chdir /build/go-lint-src/pkgs/go: no such file or directory`).
Conformist behaviour answers: conformist/rapid-dogwood/bozo. Documented in conformist(7)
TREE ROOT AND SCOPE and conformist.toml(5) GLOB PATTERNS (conformist 770fad9; the stdin
paragraph and the #132 fix landed at 400bbbd).

## 1. What happened

| commit | tree root dagnabit gives the raw conformist binary |
|---|---|
| a701f56 (May 31) | `--tree-root <outputPath>` (= `<module>/pkgs`, or `<tmp>/pkgs` under `-check`) |
| 8d3cce0 (Jun 19, #162) → Sep 30 | effectively **none**. The wrapper sniff also matched the raw ELF (#170/#195), so conformist fell back to cwd = **module root** |
| e7d4671 (Sep 30, #195) | `--tree-root <outputPath>` again |

For about three months consumers ran with tree root = module root, and their configs were
tuned to it: chrest's and cutting-garden's `pkgs/**` + `**/pkgs/**` excludes, madder's
`workingDir = ""` facade eval, and purse-first's `sed` stripping `working-dir`. e7d4671 anchored
at `pkgs/`. That stripped the path context, so the facade excludes stopped matching
(cutting-garden 40c240f: facades now gofumpt-grouped) and `working-dir = "go"` resolved to
`pkgs/go` (chrest).

The underlying problem is that dagnabit formats facades with the **repo's own** config. Any
exclude, `working-dir`, or linter in that config is written for the repo's own runs, and it
leaks into dagnabit's pass differently depending on the tree root.

## 2. Conformist facts the design relies on (all reproduced by bozo)

- `--formatters` only filters formatters. Every linter with a repair command still runs.
  Global excludes don't stop whole-tree linters. codegen-repair fires on any walked `.go`
  file and runs at the tree root. **The only formatters-only route is a config with no `[linter.*]`.**
- A formatter's `working-dir` = treeRoot/working-dir. It is only entered when that formatter has files to format.
- Globs match the path relative to the tree root. `*` and `**` cross `/`. `**/pkgs/**` does NOT match `pkgs/x.go`.
  A positional path only narrows the walk.
- `CONFORMIST_TREE_ROOT_FILE`/`_CMD` (from env or config) plus an explicit `--tree-root` is a hard error.
  `CONFORMIST_TREE_ROOT` is silently overridden by the flag.
- `--stdin` does run formatters only (no linters), but it still applies excludes and `working-dir`,
  can print partial output on failure, and hands each formatter a **temp copy outside the module**
  (goimports/gofumpt may lose `go.mod` context; unverified). **Rejected** as the primitive.
- conformist#132 is fixed (89a979b, merged to conformist master at 400bbbd): outside a git worktree
  codegen-repair warns and exits 0 unless `strict = true`. Consumers get it only once they bump
  their conformist input.

## 3. Model (user's, refined)

1. **Facades belong to the generator.** The repo's own conformist config (`nix fmt`,
   `checks.formatting`, pre-commit/repair) **excludes the generated facade files**. The two
   passes can no longer fight over them.
2. **dagnabit formats facades with a dedicated *facade config*.** It holds formatters only:
   no `[linter.*]`, no excludes, no `working-dir`. It is passed via `DAGNABIT_CONFORMIST_CONFIG`
   and run in file mode on the real files, so the formatters sit inside the module and see `go.mod`.
3. **Facade bytes = f(dagnabit version, facade config).** That makes a formatter or toolchain bump
   ordinary codegen drift, caught by the existing `-check` / `dagnabit-codegen` gates.

Considered and not taken now: formatting in-process with gofumpt/goimports as Go libraries.
That is simpler still, but facade style would be pinned by dagnabit rather than the consumer,
and rust mode would need a separate answer. It stays a possible follow-up.

## 4. dagnabit changes

1. **Tree root = `outputRoot()`**: the module root for export, the in-tree temp root under `-check`.
   Positional path = the output `pkgs/` dir. Both modes see `pkgs/...` relative paths.
   With a facade config (no excludes, no `working-dir`, no linters) the root only decides where the
   formatters run, and inside the module is right for goimports/gofumpt.
   This drops the `module | output` setting from the previous draft.
2. **Scrub `CONFORMIST_TREE_ROOT`, `CONFORMIST_TREE_ROOT_FILE`, `CONFORMIST_TREE_ROOT_CMD`** from the
   conformist child's env whenever dagnabit passes `--tree-root`.
3. **Keep the #195 fix** (only a `#!` script counts as the wrapper). Wrapper path unchanged.
4. **Document the facade-config contract** in dagnabit(1) ENVIRONMENT and in the
   `dewey-facade-export` module's `conformistConfig` description.
5. **Optional enforcement** (decision D2): dagnabit parses the facade config's top level and refuses a
   `[linter.*]` table, a formatter `working-dir`, or `excludes`. It is cheap, but it couples dagnabit to
   conformist's TOML key names.
6. Discovery path (no env var, an on-disk `conformist.toml`) is unchanged (tree root = config dir),
   but under the model that config excludes the facades, so it would format nothing. Decision D3.
7. Tests: facade config + module root; `-check` temp root sees `pkgs/`; env scrub; a
   `working-dir`-bearing config fails loud (if D2) or is documented (if not).

## 5. purse-first-provided helper

Publish `lib.conformistModules.dagnabit-facade`, a conformist module that enables only
`programs.goimports` (priority 1) and `programs.gofumpt` (priority 2). A consumer builds
its facade config with its own conformist and nixpkgs pins:

    facadeEval = conformist.lib.evalModule pkgs {
      imports = [ purse-first.lib.conformistModules.dagnabit-facade ];
      package = conformist.packages.${system}.default;
    };
    # DAGNABIT_CONFORMIST_CONFIG = facadeEval.config.build.configFile

Without the helper, each consumer hand-writes the same three lines of formatter config (madder
does today). The helper also gives the fleet one place to change facade style.

## 6. Per-consumer changes

| consumer | change | one-time regeneration? |
|---|---|---|
| **purse-first** | build the facade config from the helper and use it in the `dewey-facade-export` linter module config + justfile recipes; delete the `sed` on `working-dir`; add the facade exclude (D1) to `conformist.nix` | probably none: its facades are already goimports+gofumpt formatted. Confirm with `-check` |
| **chrest** | `dagnabitPinned`: `DAGNABIT_CONFORMIST_CONFIG` → facade config; `conformist.nix`: facade excludes become just the D1 form (`go/pkgs/**`), drop `pkgs/**` / `**/pkgs/**` | **yes**: facades go from raw to formatted (`just build-dagnabit-export`). The purse-first bump restamps them anyway |
| **cutting-garden** | `dagnabitPinned`: `DAGNABIT_CONFORMIST_CONFIG` → facade config (one line plus the eval); excludes: keep `pkgs/**`, drop `**/pkgs/**` | likely none: 40c240f already formatted them with goimports+gofumpt. Confirm with `-check`; a difference would only come from gofumpt options in its own config |
| **madder** | replace `conformistFacadeFormatEval` (currently `presets.eng` + its config with `workingDir` forced to `""`, so it still carries linters) with the helper; add the D1 exclude for `go/pkgs` | probably none. Confirm with `-check` |
| **piggy** | `conformistFacadeModule.conformistConfig` → facade config. Its current config has **no Go formatter**, so facades are formatted by nobody | **yes**: facades become goimports+gofumpt formatted. Matches its own comment's intent ("formatted by the dagnabit facade lane") |

Order and coupling: chrest can bump to the new purse-first and make its change in one commit.
cutting-garden, madder and piggy keep working on the new purse-first **before** their change,
because their old configs have no `working-dir` or already strip it. The residual #195 risk in
cutting-garden (module root = repo root with `flake.nix`, while its `presets.eng` config is still
wired) goes away with its change, or with conformist#132 merged. **So cutting-garden should make its
change in the same window as the purse-first bump**, or bump its conformist input to ≥400bbbd (#132) at the same time.
Every other consumer can migrate at its own pace.

## 7. Decisions (resolved by the user 2026-09-30)

- D1 → (a) now: exclude `<deweyDir>/pkgs/**` (accepting that hand-written `pkgs/**/*_test.go` go unformatted); generated-header skipping is requested as conformist#133 (filed, not scheduled).
- D2 → enforce: dagnabit hard-errors on `[linter.*]`, `excludes`, or a formatter `working-dir` in the facade config.
- D3 → deprecate the discovery path with a warning.
- D4 → publish `lib.conformistModules.dagnabit-facade`.
- D5 → no conformist `--no-linters` flag.

### Original options

- **D1. How the repo's own config identifies facades.** Generated names are `main.go`, build-tag files
  (`debug.go`, `not_debug.go`, `test.go`) and, in copy mode, `<leaf>.go`. Hand-written files do live
  under `pkgs/` (purse-first: `pkgs/pool/main_test.go`, `pkgs/ui/test_test.go`). Options:
  (a) exclude `<deweyDir>/pkgs/**` and accept that hand-written `pkgs/**/*_test.go` go unformatted;
  (b) exclude `<deweyDir>/pkgs/**/*.go` minus `_test.go` (gobwas supports `{}` / `[!…]`, but writing
  "not `_test.go`" as one glob is awkward); (c) ask conformist to skip files with the standard
  `// Code generated … DO NOT EDIT.` header (conformist's lane; cleanest, and it also covers tommy).
  My lean: (a) now, (c) as a conformist request.
- **D2.** Enforce the facade-config contract in dagnabit (§4.5), or only document it?
- **D3.** Discovery path: deprecate it (require `DAGNABIT_CONFORMIST_CONFIG`, warn when only an on-disk
  config is found), or leave it? Every known consumer uses the env var.
- **D4.** Publish the §5 helper, or let each consumer hand-write the formatter pair?
- **D5. Decided (user, via conformist/rapid-dogwood):** no `--no-linters` flag in conformist.
  The only formatters-only route is a config with no `[linter.*]`, which is this design's facade config.
  `--stdin`'s partial-output-on-failure also stays as it is; the design doesn't use stdin.
