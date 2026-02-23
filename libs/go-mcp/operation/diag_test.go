package operation

import "testing"

func TestDiagSetAddsToExtras(t *testing.T) {
	c := &ctx{}
	c.DiagSet("key", "value")
	if c.extras["key"] != "value" {
		t.Errorf("expected extras[key]=value, got %v", c.extras["key"])
	}
}

func TestDiagSetMultipleKeys(t *testing.T) {
	c := &ctx{}
	c.DiagSet("a", 1)
	c.DiagSet("b", 2)
	if c.extras["a"] != 1 || c.extras["b"] != 2 {
		t.Errorf("expected both keys set, got %v", c.extras)
	}
}
