# Self-Contained Marketplace Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Change marketplace generation to emit relative directory sources instead of GitHub references, making the marketplace self-contained.

**Architecture:** Add a `PluginsPrefix` field to `GenerateOptions` that specifies the relative path from marketplace root to the plugins directory. The CLI computes this from `--plugins-dir` and `--output` flags. `Generate()` uses it to build `source: "./<prefix>/<name>"` instead of GitHub URLs.

**Tech Stack:** Go, Nix

---

### Task 1: Update Generate to accept and use PluginsPrefix

**Files:**
- Modify: `internal/marketplace/generate.go:101-103` (GenerateOptions)
- Modify: `internal/marketplace/generate.go:158-166` (source resolution in Generate)

**Step 1: Write the failing test**

Add to `internal/marketplace/generate_test.go`:

```go
func TestGenerateWithPluginsPrefix(t *testing.T) {
	config := Config{
		Name:  "test-marketplace",
		Repo:  "example/test-marketplace",
		Owner: Owner{Name: "test"},
		Plugins: map[string]PluginMeta{
			"alpha": {
				Description: "Alpha MCP server",
				Version:     "1.0.0",
				Repo:        "example/alpha",
			},
		},
	}

	discovered := []DiscoveredPlugin{
		{
			Name: "alpha",
			McpServers: map[string]purse.McpServer{
				"alpha": {Type: "stdio", Command: "alpha-server"},
			},
		},
	}

	m := Generate(config, discovered, GenerateOptions{
		PluginsPrefix: "share/purse-first",
	})

	if len(m.Plugins) != 1 {
		t.Fatalf("len(plugins) = %d, want 1", len(m.Plugins))
	}

	source, ok := m.Plugins[0].Source.(string)
	if !ok {
		t.Fatalf("source type = %T, want string", m.Plugins[0].Source)
	}
	if source != "./share/purse-first/alpha" {
		t.Errorf("source = %q, want %q", source, "./share/purse-first/alpha")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/marketplace/ -run TestGenerateWithPluginsPrefix -v`
Expected: FAIL — source will be GitHubSource, not string

**Step 3: Add PluginsPrefix to GenerateOptions and use it in Generate**

In `internal/marketplace/generate.go`, add `PluginsPrefix` to `GenerateOptions`:

```go
type GenerateOptions struct {
	StripHooks    bool
	PluginsPrefix string
}
```

Replace the source resolution block (lines 158-166) with:

```go
		var source any
		switch {
		case opt.PluginsPrefix != "":
			source = "./" + opt.PluginsPrefix + "/" + dp.Name
		case meta.Repo != "":
			source = GitHubSource{Source: "github", Repo: meta.Repo}
		case config.Repo != "":
			source = GitHubSource{Source: "github", Repo: config.Repo}
		default:
			source = "."
		}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/marketplace/ -run TestGenerateWithPluginsPrefix -v`
Expected: PASS

**Step 5: Commit**

```
feat: add PluginsPrefix to Generate for relative directory sources
```

---

### Task 2: Update existing tests for the new source behavior

**Files:**
- Modify: `internal/marketplace/generate_test.go`

The existing tests that pass `Repo` without `PluginsPrefix` should still work
(GitHub source fallback). No test changes needed — just verify they pass.

**Step 1: Run all existing tests**

Run: `go test ./internal/marketplace/ -v`
Expected: All PASS — existing tests don't set PluginsPrefix, so they still use
GitHub source fallback

**Step 2: Commit (skip if no changes needed)**

---

### Task 3: Compute PluginsPrefix in the CLI command

**Files:**
- Modify: `cmd/purse-first/main.go:85-110` (genMarketplaceCmd)

**Step 1: Write the implementation**

In the `genMarketplaceCmd` RunE, compute `pluginsPrefix` from `pluginsDir` and
`outputPath`, then pass it to `Generate`:

```go
		// Compute the relative path from the marketplace root to the
		// plugins directory. The marketplace root is the parent of
		// .claude-plugin/ (which contains the output file).
		outputDir := filepath.Dir(outputPath)           // .claude-plugin
		marketplaceRoot := filepath.Dir(outputDir)      // $out
		pluginsPrefix, err := filepath.Rel(marketplaceRoot, pluginsDir)
		if err != nil {
			return fmt.Errorf("computing plugins prefix: %w", err)
		}

		m := marketplace.Generate(config, discovered, marketplace.GenerateOptions{
			StripHooks:    genNoHooks,
			PluginsPrefix: pluginsPrefix,
		})
```

**Step 2: Build to verify it compiles**

Run: `go build ./cmd/purse-first/`
Expected: Success

**Step 3: Commit**

```
feat: compute PluginsPrefix from CLI flags for self-contained marketplace
```

---

### Task 4: Clean up marketplace-config.json

**Files:**
- Modify: `marketplace-config.json`

**Step 1: Remove per-plugin `repo` fields**

The per-plugin `repo` fields are now unused for source resolution (PluginsPrefix
takes precedence). Remove them to avoid confusion. Keep the top-level `repo` for
homepage/metadata.

Remove `"repo": "amarbel-llc/purse-first"` from each plugin entry in the
`plugins` object. The `homepage` field already provides the URL.

**Step 2: Commit**

```
chore: remove per-plugin repo fields from marketplace-config.json
```

---

### Task 5: Rebuild and verify end-to-end

**Step 1: Build with Nix**

Run: `just build-nix`

**Step 2: Verify marketplace.json has relative directory sources**

Run: `cat result/.claude-plugin/marketplace.json | python3 -c "import json,sys; m=json.load(sys.stdin); [print(f'{p[\"name\"]}: {json.dumps(p[\"source\"])}') for p in m['plugins']]"`

Expected output:
```
chix: "./share/purse-first/chix"
get-hubbed: "./share/purse-first/get-hubbed"
grit: "./share/purse-first/grit"
lux: "./share/purse-first/lux"
purse-first: "./share/purse-first/purse-first"
robin: "./share/purse-first/robin"
tap-dancer: "./share/purse-first/tap-dancer"
```

**Step 3: Reinstall and verify MCPs appear**

Run: `purse-first install`
Then start a new Claude Code session and run `/mcp` — MCP servers should now
appear.

**Step 4: Commit (if any fixups needed)**

---

### Task 6: Run full test suite

**Step 1: Run all tests**

Run: `just test`
Expected: All pass

**Step 2: Final commit if needed**
