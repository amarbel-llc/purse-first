package command

import "testing"

func TestAppAddCommand(t *testing.T) {
	app := NewUtility("grit", "Git operations MCP server")

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
	app := NewUtility("dodder", "Zettelkasten CLI")

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
	app := NewUtility("test", "test")
	app.AddCommand(&Command{Name: "foo"})

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate command")
		}
	}()

	app.AddCommand(&Command{Name: "foo"})
}

func TestAppMergeWithPrefix(t *testing.T) {
	parent := NewUtility("dodder", "main")
	child := NewUtility("madder", "blob store")

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
	app := NewUtility("test", "test")
	app.AddCommand(&Command{Name: "a"})
	app.AddCommand(&Command{Name: "b"})
	app.AddCommand(&Command{Name: "c", Hidden: true})

	count := 0
	for range app.AllCommands() {
		count++
	}
	if count != 5 {
		t.Errorf("AllCommands count = %d, want 5", count)
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
	app := NewUtility("test", "test")
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
	app := NewUtility("test", "test")
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

func TestAppMergeWithPrefixAllCommandsYieldsPrefixedName(t *testing.T) {
	parent := NewUtility("dodder", "main")
	child := NewUtility("perms", "permissions")

	child.AddCommand(&Command{Name: "list"})
	child.AddCommand(&Command{Name: "grant"})

	parent.MergeWithPrefix(child, "perms")

	found := make(map[string]bool)
	for name := range parent.AllCommands() {
		found[name] = true
	}

	for _, want := range []string{"perms-list", "perms-grant"} {
		if !found[want] {
			t.Errorf("AllCommands missing prefixed name %q, got %v", want, found)
		}
	}

	for name := range found {
		if name == "list" || name == "grant" {
			t.Errorf("AllCommands yielded unprefixed name %q", name)
		}
	}
}

func TestUtilityGetName(t *testing.T) {
	app := NewUtility("grit", "Git operations MCP server")
	if got := app.GetName(); got != "grit" {
		t.Errorf("GetName() = %q, want %q", got, "grit")
	}
}

// --- AddCmd tests ---

// stubCmd implements only Cmd (no optional interfaces).
type stubCmd struct{}

func (stubCmd) Run(Request) {}

// describedCmd implements Cmd + CommandWithDescription.
type describedCmd struct{}

func (describedCmd) Run(Request) {}
func (describedCmd) GetDescription() Description {
	return Description{Short: "A described command", Long: "Longer description here."}
}

// paramCmd implements Cmd + CommandWithDescription + CommandWithParams.
type paramCmd struct{}

func (paramCmd) Run(Request) {}
func (paramCmd) GetDescription() Description {
	return Description{Short: "Parameterized command"}
}
func (paramCmd) GetParams() []Param {
	return []Param{
		StringFlag{Name: "path", Description: "File path", Required: true, Short: 'p'},
		BoolFlag{Name: "verbose", Description: "Verbose output", Short: 'v'},
	}
}

// resultCmd implements Cmd + CommandWithResult.
type resultCmd struct{}

func (resultCmd) Run(Request) {}
func (resultCmd) GetDescription() Description {
	return Description{Short: "Returns a result"}
}
func (resultCmd) RunResult(req Request) (*Result, error) {
	return TextResult("hello"), nil
}

func TestAddCmdBasic(t *testing.T) {
	app := NewUtility("test", "test")
	app.AddCmd("do-thing", stubCmd{})

	cmd, ok := app.GetCommand("do-thing")
	if !ok {
		t.Fatal("GetCommand(do-thing) not found")
	}
	if cmd.Name != "do-thing" {
		t.Errorf("cmd.Name = %q, want %q", cmd.Name, "do-thing")
	}
	// Run is always set — AddCmd bridges plain Cmd to Command.Run
	if cmd.Run == nil {
		t.Error("stubCmd should have Run set via AddCmd bridge")
	}
}

func TestAddCmdWithDescription(t *testing.T) {
	app := NewUtility("test", "test")
	app.AddCmd("described", describedCmd{})

	cmd, _ := app.GetCommand("described")
	if cmd.Description.Short != "A described command" {
		t.Errorf("Description.Short = %q", cmd.Description.Short)
	}
	if cmd.Description.Long != "Longer description here." {
		t.Errorf("Description.Long = %q", cmd.Description.Long)
	}
}

func TestAddCmdWithParams(t *testing.T) {
	app := NewUtility("test", "test")
	app.AddCmd("parameterized", paramCmd{})

	cmd, _ := app.GetCommand("parameterized")
	if len(cmd.Params) != 2 {
		t.Fatalf("Params len = %d, want 2", len(cmd.Params))
	}
	if cmd.Params[0].paramName() != "path" {
		t.Errorf("Params[0].paramName() = %q, want %q", cmd.Params[0].paramName(), "path")
	}
	if !cmd.Params[0].paramRequired() {
		t.Error("Params[0] should be required")
	}
}

func TestAddCmdWithResult(t *testing.T) {
	app := NewUtility("test", "test")
	app.AddCmd("greet", resultCmd{})

	cmd, _ := app.GetCommand("greet")
	if cmd.Run == nil {
		t.Fatal("resultCmd should have Run set")
	}

	// Invoke the Run handler
	result, err := cmd.Run(t.Context(), nil, StubPrompter{})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.Text != "hello" {
		t.Errorf("result.Text = %q, want %q", result.Text, "hello")
	}
}

func TestAddCmdSetsUtilityOnRequest(t *testing.T) {
	app := NewUtility("madder", "blob store")

	var gotUtility *Utility
	app.AddCmd("cat", &captureCmd{onRun: func(req Request) {
		gotUtility = req.Utility
	}})

	cmd, _ := app.GetCommand("cat")
	if cmd.Run == nil {
		t.Fatal("plain Cmd should have Run set via AddCmd")
	}

	_, _ = cmd.Run(t.Context(), nil, StubPrompter{})
	if gotUtility == nil {
		t.Fatal("req.Utility was nil")
	}
	if gotUtility.GetName() != "madder" {
		t.Errorf("req.Utility.GetName() = %q, want %q", gotUtility.GetName(), "madder")
	}
}

func TestAddCmdPlainCmdHasRun(t *testing.T) {
	app := NewUtility("test", "test")

	var ran bool
	app.AddCmd("do-thing", &captureCmd{onRun: func(req Request) {
		ran = true
	}})

	cmd, _ := app.GetCommand("do-thing")
	if cmd.Run == nil {
		t.Fatal("plain Cmd should have Run set")
	}

	_, _ = cmd.Run(t.Context(), nil, StubPrompter{})
	if !ran {
		t.Error("Cmd.Run was not called")
	}
}

func TestAddCmdPopulatesInputFromJSON(t *testing.T) {
	app := NewUtility("test", "test")

	var gotArg string
	cmd := &captureParamCmd{
		params: []Param{
			StringArg{Name: "path", Description: "File path", Required: true},
		},
		onRun: func(req Request) {
			gotArg = req.PopArg("path")
		},
	}
	app.AddCmd("open", cmd)

	c, _ := app.GetCommand("open")
	argsJSON := []byte(`{"path": "/tmp/foo"}`)
	_, err := c.Run(t.Context(), argsJSON, StubPrompter{})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if gotArg != "/tmp/foo" {
		t.Errorf("PopArg(path) = %q, want %q", gotArg, "/tmp/foo")
	}
}

// captureCmd implements Cmd and captures the Request for assertions.
type captureCmd struct {
	onRun func(Request)
}

func (c *captureCmd) Run(req Request) {
	if c.onRun != nil {
		c.onRun(req)
	}
}

// captureParamCmd implements Cmd + CommandWithParams.
type captureParamCmd struct {
	params []Param
	onRun  func(Request)
}

func (c *captureParamCmd) Run(req Request) {
	if c.onRun != nil {
		c.onRun(req)
	}
}

func (c *captureParamCmd) GetParams() []Param {
	return c.params
}

func TestAddCmdPanicsOnDuplicate(t *testing.T) {
	app := NewUtility("test", "test")
	app.AddCmd("foo", stubCmd{})

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate AddCmd")
		}
	}()

	app.AddCmd("foo", stubCmd{})
}
