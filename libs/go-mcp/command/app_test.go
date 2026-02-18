package command

import "testing"

func TestAppAddCommand(t *testing.T) {
	app := NewApp("grit", "Git operations MCP server")

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
	})

	cmd, ok := app.GetCommand("status")
	if !ok {
		t.Fatal("GetCommand(status) not found")
	}
	if cmd.Name != "status" {
		t.Errorf("cmd.Name = %q, want %q", cmd.Name, "status")
	}
}

func TestAppAddCommandWithAliases(t *testing.T) {
	app := NewApp("dodder", "Zettelkasten CLI")

	app.AddCommand(&Command{
		Name:    "checkin",
		Aliases: []string{"add", "save"},
	})

	for _, name := range []string{"checkin", "add", "save"} {
		if _, ok := app.GetCommand(name); !ok {
			t.Errorf("GetCommand(%q) not found", name)
		}
	}
}

func TestAppAddCommandPanicsOnDuplicate(t *testing.T) {
	app := NewApp("test", "test")
	app.AddCommand(&Command{Name: "foo"})

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate command")
		}
	}()

	app.AddCommand(&Command{Name: "foo"})
}

func TestAppMergeWithPrefix(t *testing.T) {
	parent := NewApp("dodder", "main")
	child := NewApp("madder", "blob store")

	child.AddCommand(&Command{Name: "cat"})
	child.AddCommand(&Command{Name: "ls"})

	parent.MergeWithPrefix(child, "blob_store")

	if _, ok := parent.GetCommand("blob_store-cat"); !ok {
		t.Error("GetCommand(blob_store-cat) not found")
	}
	if _, ok := parent.GetCommand("blob_store-ls"); !ok {
		t.Error("GetCommand(blob_store-ls) not found")
	}
}

func TestAppAllCommands(t *testing.T) {
	app := NewApp("test", "test")
	app.AddCommand(&Command{Name: "a"})
	app.AddCommand(&Command{Name: "b"})
	app.AddCommand(&Command{Name: "c", Hidden: true})

	count := 0
	for range app.AllCommands() {
		count++
	}
	if count != 4 {
		t.Errorf("AllCommands count = %d, want 4", count)
	}

	visible := 0
	for range app.VisibleCommands() {
		visible++
	}
	if visible != 2 {
		t.Errorf("VisibleCommands count = %d, want 2", visible)
	}
}

func TestAppAllCommandsYieldsCanonicalName(t *testing.T) {
	app := NewApp("test", "test")
	app.AddCommand(&Command{
		Name:    "checkin",
		Aliases: []string{"add", "save"},
	})

	for name, cmd := range app.AllCommands() {
		if name != cmd.Name {
			t.Errorf("AllCommands yielded name %q, want canonical %q", name, cmd.Name)
		}
	}
}

func TestAppVisibleCommandsYieldsCanonicalName(t *testing.T) {
	app := NewApp("test", "test")
	app.AddCommand(&Command{
		Name:    "status",
		Aliases: []string{"st"},
	})

	for name, cmd := range app.VisibleCommands() {
		if name != cmd.Name {
			t.Errorf("VisibleCommands yielded name %q, want canonical %q", name, cmd.Name)
		}
	}
}
