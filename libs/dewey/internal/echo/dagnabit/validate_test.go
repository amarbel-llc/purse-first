package dagnabit

import (
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

const testModulePath = "github.com/example/mod"

func mkPkgs(paths ...string) []*packages.Package {
	out := make([]*packages.Package, 0, len(paths))
	for _, p := range paths {
		out = append(out, &packages.Package{PkgPath: p})
	}
	return out
}

func TestValidateUniqueLeavesPkgs(t *testing.T) {
	t.Run("all unique", func(t *testing.T) {
		err := validateUniqueLeavesPkgs(mkPkgs(
			testModulePath+"/alfa/cmp",
			testModulePath+"/bravo/errors",
			testModulePath+"/echo/dagnabit",
		), testModulePath)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("single collision", func(t *testing.T) {
		err := validateUniqueLeavesPkgs(mkPkgs(
			testModulePath+"/alfa/foo",
			testModulePath+"/bravo/foo",
		), testModulePath)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		msg := err.Error()
		if !strings.Contains(msg, "alfa/foo") || !strings.Contains(msg, "bravo/foo") {
			t.Errorf("error should name both colliding paths, got: %s", msg)
		}
		if !strings.Contains(msg, "foo:") {
			t.Errorf("error should name the colliding leaf, got: %s", msg)
		}
	})

	t.Run("three-way collision", func(t *testing.T) {
		err := validateUniqueLeavesPkgs(mkPkgs(
			testModulePath+"/alfa/foo",
			testModulePath+"/bravo/foo",
			testModulePath+"/echo/foo",
		), testModulePath)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		msg := err.Error()
		for _, want := range []string{"alfa/foo", "bravo/foo", "echo/foo"} {
			if !strings.Contains(msg, want) {
				t.Errorf("error should name %q, got: %s", want, msg)
			}
		}
	})

	t.Run("same node listed twice does not collide", func(t *testing.T) {
		err := validateUniqueLeavesPkgs(mkPkgs(
			testModulePath+"/alfa/cmp",
			testModulePath+"/alfa/cmp", // duplicate — same node, not a collision
		), testModulePath)
		if err != nil {
			t.Errorf("duplicate of the same node is not a collision, got: %v", err)
		}
	})

	t.Run("ignores out-of-module packages", func(t *testing.T) {
		err := validateUniqueLeavesPkgs(mkPkgs(
			testModulePath+"/alfa/cmp",
			"github.com/external/repo/alfa/cmp", // external, ignored
		), testModulePath)
		if err != nil {
			t.Errorf("external package should be ignored, got: %v", err)
		}
	})
}

func TestValidateTwoLayerLayoutPkgs(t *testing.T) {
	t.Run("all 2-layer", func(t *testing.T) {
		err := validateTwoLayerLayoutPkgs(mkPkgs(
			testModulePath+"/alfa/cmp",
			testModulePath+"/echo/dagnabit",
			testModulePath+"/cmd/foo",
		), testModulePath)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("sub-package flagged", func(t *testing.T) {
		err := validateTwoLayerLayoutPkgs(mkPkgs(
			testModulePath+"/echo/command",     // OK, 2-layer
			testModulePath+"/echo/command/huh", // VIOLATION, 3-layer
		), testModulePath)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		msg := err.Error()
		if !strings.Contains(msg, "echo/command/huh") {
			t.Errorf("error should name the offending path, got: %s", msg)
		}
		if strings.Contains(msg, "echo/command\n") || strings.HasSuffix(strings.TrimSpace(msg), "echo/command") {
			t.Errorf("error should NOT flag the parent (2-layer) package, got: %s", msg)
		}
	})

	t.Run("ignores out-of-module", func(t *testing.T) {
		err := validateTwoLayerLayoutPkgs(mkPkgs(
			testModulePath+"/alfa/cmp",
			"github.com/external/repo/foo/bar/baz", // 3-component external, ignored
		), testModulePath)
		if err != nil {
			t.Errorf("external package should be ignored, got: %v", err)
		}
	})

	t.Run("ignores root-level packages", func(t *testing.T) {
		err := validateTwoLayerLayoutPkgs(mkPkgs(
			testModulePath+"/main", // 1-component, ignored
			testModulePath+"/alfa/cmp",
		), testModulePath)
		if err != nil {
			t.Errorf("root-level package should be ignored, got: %v", err)
		}
	})
}
