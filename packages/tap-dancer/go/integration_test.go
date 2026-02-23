package tap

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/operation"
)

func TestEndToEndTAPOutput(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	ow := NewOperationWriter(tw)
	ctx := operation.New(ow)

	ctx.Run("deploy", func(ctx operation.Context) error {
		ctx.Run("backup", func(ctx operation.Context) error {
			ctx.DiagSet("size_mb", 420)
			return nil
		}, operation.ReadOnly)

		ctx.Run("migrate", func(ctx operation.Context) error {
			return ctx.ControlWrap(errors.New("pq: relation exists"))
		}, operation.Destructive, operation.Idempotent)

		return nil
	})
	tw.Plan()

	out := buf.String()
	t.Logf("TAP output:\n%s", out)

	if !strings.Contains(out, "TAP version 14") {
		t.Error("expected TAP version 14 header")
	}
	if !strings.Contains(out, "# Subtest: deploy") {
		t.Error("expected subtest header for deploy")
	}
	if !strings.Contains(out, "ok 1 - backup") {
		t.Error("expected ok for backup")
	}
	if !strings.Contains(out, "size_mb: 420") {
		t.Error("expected size_mb diagnostic on backup")
	}
	if !strings.Contains(out, "not ok 2 - migrate") {
		t.Error("expected not ok for migrate")
	}
	if !strings.Contains(out, "source: external") {
		t.Error("expected source: external on migrate")
	}
	if !strings.Contains(out, "pq: relation exists") {
		t.Error("expected error message in migrate diagnostic")
	}
	// Parent succeeds because ControlWrap is contained within the child.
	// The child's Run catches failSentinel internally and does not propagate
	// it as retErr, so the parent's fn continues and returns nil.
	if !strings.Contains(out, "ok 1 - deploy") {
		t.Error("expected ok for deploy (child failure is contained)")
	}
}

func TestEndToEndMustFailure(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	ow := NewOperationWriter(tw)
	ctx := operation.New(ow)

	ctx.Run("step", func(ctx operation.Context) error {
		ctx.Must(func() error { return errors.New("flush failed") })
		return nil
	})
	tw.Plan()

	out := buf.String()
	t.Logf("TAP output:\n%s", out)

	if !strings.Contains(out, "not ok 1 - step") {
		t.Error("expected not ok when Must fails")
	}
	if !strings.Contains(out, "flush failed") {
		t.Error("expected must error message in output")
	}
}

func TestEndToEndSkip(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	ow := NewOperationWriter(tw)
	ctx := operation.New(ow)

	ctx.Run("optional", func(ctx operation.Context) error {
		return ctx.ControlSkip("not applicable")
	})
	tw.Plan()

	out := buf.String()
	t.Logf("TAP output:\n%s", out)

	if !strings.Contains(out, "# SKIP not applicable") {
		t.Error("expected skip directive")
	}
}
