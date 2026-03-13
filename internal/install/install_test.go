package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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
