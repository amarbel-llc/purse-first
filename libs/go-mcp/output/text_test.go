package output

import (
	"strings"
	"testing"
)

func TestLimitTextNoTruncation(t *testing.T) {
	result := LimitText("hello\nworld\n", TextLimits{})
	if result.Truncated {
		t.Fatal("expected no truncation with zero limits")
	}

	if result.Content != "hello\nworld\n" {
		t.Fatalf("expected original content, got %q", result.Content)
	}

	if result.TruncationInfo != nil {
		t.Fatal("expected nil TruncationInfo when not truncated")
	}
}

func TestLimitTextEmptyInput(t *testing.T) {
	result := LimitText("", TextLimits{Head: 5})
	if result.Truncated {
		t.Fatal("expected no truncation for empty input")
	}

	if result.Content != "" {
		t.Fatalf("expected empty content, got %q", result.Content)
	}
}

func TestLimitTextHead(t *testing.T) {
	input := "line1\nline2\nline3\nline4\nline5\n"
	result := LimitText(input, TextLimits{Head: 2})

	if !result.Truncated {
		t.Fatal("expected truncation")
	}

	if result.Content != "line1\nline2" {
		t.Fatalf("expected first 2 lines, got %q", result.Content)
	}

	if result.TruncationInfo.Position != "head" {
		t.Fatalf("expected position head, got %q", result.TruncationInfo.Position)
	}

	if result.TruncationInfo.OriginalLines != 5 {
		t.Fatalf("expected 5 original lines, got %d", result.TruncationInfo.OriginalLines)
	}

	if result.TruncationInfo.KeptLines != 2 {
		t.Fatalf("expected 2 kept lines, got %d", result.TruncationInfo.KeptLines)
	}
}

func TestLimitTextTail(t *testing.T) {
	input := "line1\nline2\nline3\nline4\nline5"
	result := LimitText(input, TextLimits{Tail: 2})

	if !result.Truncated {
		t.Fatal("expected truncation")
	}

	if result.Content != "line4\nline5" {
		t.Fatalf("expected last 2 lines, got %q", result.Content)
	}

	if result.TruncationInfo.Position != "tail" {
		t.Fatalf("expected position tail, got %q", result.TruncationInfo.Position)
	}
}

func TestLimitTextHeadPriorityOverTail(t *testing.T) {
	input := "line1\nline2\nline3\nline4\nline5"
	result := LimitText(input, TextLimits{Head: 2, Tail: 2})

	if !result.Truncated {
		t.Fatal("expected truncation")
	}

	if result.Content != "line1\nline2" {
		t.Fatalf("expected head to win, got %q", result.Content)
	}

	if result.TruncationInfo.Position != "head" {
		t.Fatalf("expected position head, got %q", result.TruncationInfo.Position)
	}
}

func TestLimitTextMaxLines(t *testing.T) {
	input := "a\nb\nc\nd\ne"
	result := LimitText(input, TextLimits{MaxLines: 3})

	if !result.Truncated {
		t.Fatal("expected truncation")
	}

	if result.Content != "a\nb\nc" {
		t.Fatalf("expected 3 lines, got %q", result.Content)
	}

	if result.TruncationInfo.KeptLines != 3 {
		t.Fatalf("expected 3 kept lines, got %d", result.TruncationInfo.KeptLines)
	}
}

func TestLimitTextMaxLinesWithTail(t *testing.T) {
	input := "a\nb\nc\nd\ne"
	result := LimitText(input, TextLimits{Tail: 4, MaxLines: 2})

	if !result.Truncated {
		t.Fatal("expected truncation")
	}

	// Tail selects last 4: b,c,d,e → MaxLines keeps last 2: d,e
	if result.Content != "d\ne" {
		t.Fatalf("expected last 2 of tail selection, got %q", result.Content)
	}

	if result.TruncationInfo.Position != "tail" {
		t.Fatalf("expected position tail, got %q", result.TruncationInfo.Position)
	}
}

func TestLimitTextMaxBytesAtLineBoundary(t *testing.T) {
	input := "short\nmedium line\nlong line here"
	// "short\nmedium line\n" = 18 bytes — set limit to 20 to cut at the second newline
	result := LimitText(input, TextLimits{MaxBytes: 20})

	if !result.Truncated {
		t.Fatal("expected truncation")
	}

	// Should cut at the last newline within the 20-byte window
	if result.Content != "short\nmedium line\n" {
		t.Fatalf("expected truncation at line boundary, got %q", result.Content)
	}
}

func TestLimitTextMaxBytesUTF8Safety(t *testing.T) {
	// 3-byte UTF-8 rune: € = 0xE2 0x82 0xAC
	input := "abc€def"
	// "abc" = 3 bytes, "€" = 3 bytes → byte 4 and 5 are mid-rune
	result := LimitText(input, TextLimits{MaxBytes: 5})

	if !result.Truncated {
		t.Fatal("expected truncation")
	}

	// Should back up to avoid splitting the € rune: keep "abc" (3 bytes)
	if result.Content != "abc" {
		t.Fatalf("expected UTF-8 safe truncation to %q, got %q", "abc", result.Content)
	}
}

func TestLimitTextCompoundLimits(t *testing.T) {
	input := "aaaa\nbbbb\ncccc\ndddd\neeee"
	result := LimitText(input, TextLimits{Head: 4, MaxLines: 3, MaxBytes: 12})

	if !result.Truncated {
		t.Fatal("expected truncation")
	}

	// Head 4 → aaaa,bbbb,cccc,dddd
	// MaxLines 3 → aaaa,bbbb,cccc
	// Rejoined: "aaaa\nbbbb\ncccc" = 14 bytes > 12
	// MaxBytes 12 → truncate at line boundary → "aaaa\nbbbb\n" = 10 bytes
	if result.Content != "aaaa\nbbbb\n" {
		t.Fatalf("expected compound truncation, got %q (len=%d)", result.Content, len(result.Content))
	}
}

func TestLimitTextTrailingNewline(t *testing.T) {
	input := "hello\nworld\n"
	result := LimitText(input, TextLimits{})

	if result.Truncated {
		t.Fatal("expected no truncation")
	}

	if result.Content != "hello\nworld\n" {
		t.Fatalf("expected trailing newline preserved, got %q", result.Content)
	}
}

func TestLimitTextSingleLine(t *testing.T) {
	result := LimitText("hello", TextLimits{Head: 5})

	if result.Truncated {
		t.Fatal("expected no truncation for single line within head limit")
	}

	if result.Content != "hello" {
		t.Fatalf("expected hello, got %q", result.Content)
	}
}

func TestLimitTextZeroLimitsPassthrough(t *testing.T) {
	input := "some content\nwith lines\n"
	result := LimitText(input, TextLimits{})

	if result.Truncated {
		t.Fatal("zero limits should not truncate")
	}

	if result.Content != input {
		t.Fatalf("expected passthrough, got %q", result.Content)
	}
}

func TestLimitTextTruncationInfoAccuracy(t *testing.T) {
	input := "line1\nline2\nline3"
	result := LimitText(input, TextLimits{Head: 1})

	if !result.Truncated {
		t.Fatal("expected truncation")
	}

	info := result.TruncationInfo
	if info == nil {
		t.Fatal("expected TruncationInfo")
	}

	if info.OriginalBytes != len(input) {
		t.Fatalf("expected OriginalBytes=%d, got %d", len(input), info.OriginalBytes)
	}

	if info.OriginalLines != 3 {
		t.Fatalf("expected OriginalLines=3, got %d", info.OriginalLines)
	}

	if info.KeptBytes != len("line1") {
		t.Fatalf("expected KeptBytes=%d, got %d", len("line1"), info.KeptBytes)
	}

	if info.KeptLines != 1 {
		t.Fatalf("expected KeptLines=1, got %d", info.KeptLines)
	}
}

func TestLimitTextHeadExceedsLineCount(t *testing.T) {
	input := "a\nb"
	result := LimitText(input, TextLimits{Head: 10})

	if result.Truncated {
		t.Fatal("head larger than line count should not truncate")
	}

	if result.Content != input {
		t.Fatalf("expected original content, got %q", result.Content)
	}
}

func TestLimitTextTailExceedsLineCount(t *testing.T) {
	input := "a\nb"
	result := LimitText(input, TextLimits{Tail: 10})

	if result.Truncated {
		t.Fatal("tail larger than line count should not truncate")
	}

	if result.Content != input {
		t.Fatalf("expected original content, got %q", result.Content)
	}
}

func TestLimitTextMaxBytesExactFit(t *testing.T) {
	input := "hello"
	result := LimitText(input, TextLimits{MaxBytes: 5})

	if result.Truncated {
		t.Fatal("max bytes equal to content length should not truncate")
	}

	if result.Content != input {
		t.Fatalf("expected original content, got %q", result.Content)
	}
}

func TestTruncateUTF8(t *testing.T) {
	// "héllo" — é is 2 bytes (0xC3 0xA9)
	input := "héllo"

	// Cutting at byte 2 lands in the middle of é
	got := truncateUTF8(input, 2)
	if got != "h" {
		t.Fatalf("expected %q, got %q", "h", got)
	}

	// Cutting at byte 3 lands after é
	got = truncateUTF8(input, 3)
	if got != "hé" {
		t.Fatalf("expected %q, got %q", "hé", got)
	}
}

func TestLimitTextLargeInput(t *testing.T) {
	lines := make([]string, 5000)
	for i := range lines {
		lines[i] = "line content here"
	}

	input := strings.Join(lines, "\n")
	result := LimitText(input, TextLimits{Head: 100, MaxBytes: 1000})

	if !result.Truncated {
		t.Fatal("expected truncation")
	}

	if len(result.Content) > 1000 {
		t.Fatalf("expected content <= 1000 bytes, got %d", len(result.Content))
	}
}
