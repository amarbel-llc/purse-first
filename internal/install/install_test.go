package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRootFromPluginsDir(t *testing.T) {
	t.Run("resolves two levels up from plugins dir", func(t *testing.T) {
		root := rootFromPluginsDir("/nix/store/abc123-marketplace/share/purse-first")
		expected := "/nix/store/abc123-marketplace"
		if root != expected {
			t.Errorf("got %q, want %q", root, expected)
		}
	})
}

func TestResolveMarketplaceRoot(t *testing.T) {
	t.Run("finds marketplace.json via PURSE_FIRST_PLUGINS_DIR", func(t *testing.T) {
		// Create a temp directory mimicking the Nix store layout:
		//   <root>/.claude-plugin/marketplace.json
		//   <root>/share/purse-first/
		tmp := t.TempDir()
		pluginsDir := filepath.Join(tmp, "share", "purse-first")
		marketplaceDir := filepath.Join(tmp, ".claude-plugin")

		if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(marketplaceDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			filepath.Join(marketplaceDir, "marketplace.json"),
			[]byte(`{"name":"test","plugins":[]}`),
			0o644,
		); err != nil {
			t.Fatal(err)
		}

		t.Setenv("PURSE_FIRST_PLUGINS_DIR", pluginsDir)

		root, err := resolveMarketplaceRoot()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if root != tmp {
			t.Errorf("got root %q, want %q", root, tmp)
		}
	})

	t.Run("fails when marketplace.json is missing", func(t *testing.T) {
		tmp := t.TempDir()
		pluginsDir := filepath.Join(tmp, "share", "purse-first")
		if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
			t.Fatal(err)
		}

		t.Setenv("PURSE_FIRST_PLUGINS_DIR", pluginsDir)

		_, err := resolveMarketplaceRoot()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestReadMarketplace(t *testing.T) {
	t.Run("parses marketplace.json", func(t *testing.T) {
		tmp := t.TempDir()
		marketplaceDir := filepath.Join(tmp, ".claude-plugin")
		if err := os.MkdirAll(marketplaceDir, 0o755); err != nil {
			t.Fatal(err)
		}

		m := marketplaceJSON{
			Name: "test-marketplace",
			Plugins: []marketplacePlugin{
				{Name: "plugin-a"},
				{Name: "plugin-b"},
			},
		}
		data, _ := json.Marshal(m)
		if err := os.WriteFile(filepath.Join(marketplaceDir, "marketplace.json"), data, 0o644); err != nil {
			t.Fatal(err)
		}

		got, err := readMarketplace(tmp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Name != "test-marketplace" {
			t.Errorf("got name %q, want %q", got.Name, "test-marketplace")
		}
		if len(got.Plugins) != 2 {
			t.Errorf("got %d plugins, want 2", len(got.Plugins))
		}
	})
}
