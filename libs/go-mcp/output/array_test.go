package output

import "testing"

func TestLimitArrayNoTruncation(t *testing.T) {
	items := []int{1, 2, 3}
	result := LimitArray(items, ArrayLimits{})

	if result.Truncated {
		t.Fatal("expected no truncation with zero limits")
	}

	if len(result.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result.Items))
	}

	if result.TotalCount != 3 {
		t.Fatalf("expected total count 3, got %d", result.TotalCount)
	}
}

func TestLimitArrayEmptySlice(t *testing.T) {
	result := LimitArray([]int{}, ArrayLimits{Limit: 10})

	if result.Truncated {
		t.Fatal("expected no truncation for empty slice")
	}

	if len(result.Items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(result.Items))
	}

	if result.TotalCount != 0 {
		t.Fatalf("expected total count 0, got %d", result.TotalCount)
	}

	if result.Pagination.HasMore {
		t.Fatal("expected HasMore=false for empty slice")
	}
}

func TestLimitArrayLimit(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	result := LimitArray(items, ArrayLimits{Limit: 3})

	if !result.Truncated {
		t.Fatal("expected truncation")
	}

	if len(result.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result.Items))
	}

	if result.Items[0] != 1 || result.Items[2] != 3 {
		t.Fatalf("expected first 3 items, got %v", result.Items)
	}

	if !result.Pagination.HasMore {
		t.Fatal("expected HasMore=true")
	}

	if result.TotalCount != 5 {
		t.Fatalf("expected total count 5, got %d", result.TotalCount)
	}
}

func TestLimitArrayOffsetAndLimit(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	result := LimitArray(items, ArrayLimits{Offset: 2, Limit: 2})

	if !result.Truncated {
		t.Fatal("expected truncation")
	}

	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}

	if result.Items[0] != 3 || result.Items[1] != 4 {
		t.Fatalf("expected items [3,4], got %v", result.Items)
	}

	if !result.Pagination.HasMore {
		t.Fatal("expected HasMore=true")
	}

	if result.Pagination.Offset != 2 {
		t.Fatalf("expected offset 2, got %d", result.Pagination.Offset)
	}
}

func TestLimitArrayOffsetAtEnd(t *testing.T) {
	items := []int{1, 2, 3}
	result := LimitArray(items, ArrayLimits{Offset: 3})

	if len(result.Items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(result.Items))
	}

	if result.Pagination.HasMore {
		t.Fatal("expected HasMore=false")
	}
}

func TestLimitArrayOffsetBeyondEnd(t *testing.T) {
	items := []int{1, 2, 3}
	result := LimitArray(items, ArrayLimits{Offset: 100})

	if len(result.Items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(result.Items))
	}

	if result.Pagination.Offset != 3 {
		t.Fatalf("expected clamped offset 3, got %d", result.Pagination.Offset)
	}
}

func TestLimitArrayZeroLimitUnlimited(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	result := LimitArray(items, ArrayLimits{Limit: 0})

	if result.Truncated {
		t.Fatal("zero limit should mean unlimited")
	}

	if len(result.Items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(result.Items))
	}
}

func TestLimitArrayWithStrings(t *testing.T) {
	items := []string{"alpha", "bravo", "charlie", "delta"}
	result := LimitArray(items, ArrayLimits{Limit: 2})

	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}

	if result.Items[0] != "alpha" || result.Items[1] != "bravo" {
		t.Fatalf("expected [alpha, bravo], got %v", result.Items)
	}
}

type testStruct struct {
	Name  string
	Value int
}

func TestLimitArrayWithStructs(t *testing.T) {
	items := []testStruct{
		{Name: "a", Value: 1},
		{Name: "b", Value: 2},
		{Name: "c", Value: 3},
	}

	result := LimitArray(items, ArrayLimits{Offset: 1, Limit: 1})

	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}

	if result.Items[0].Name != "b" {
		t.Fatalf("expected item b, got %v", result.Items[0])
	}

	if !result.Pagination.HasMore {
		t.Fatal("expected HasMore=true")
	}
}

func TestLimitArrayPaginationInfo(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	result := LimitArray(items, ArrayLimits{Offset: 3, Limit: 4})

	p := result.Pagination
	if p.Offset != 3 {
		t.Fatalf("expected offset 3, got %d", p.Offset)
	}

	if p.Limit != 4 {
		t.Fatalf("expected limit 4, got %d", p.Limit)
	}

	if p.Total != 10 {
		t.Fatalf("expected total 10, got %d", p.Total)
	}

	if !p.HasMore {
		t.Fatal("expected HasMore=true (items 8,9,10 remain)")
	}
}
