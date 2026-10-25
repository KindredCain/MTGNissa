package setlist

import (
	"reflect"
	"testing"
	"time"
)

func testSetCode(value string) *string {
	return &value
}

func testSetDate(year, month, day int) *time.Time {
	value := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return &value
}

func TestHierarchyExpandsWholeGroupInSiblingOrder(t *testing.T) {
	links := []hierarchyRow{
		{Code: "A"}, {Code: "B"},
		{Code: "C0", ParentCode: testSetCode("A")},
		{Code: "GC", ParentCode: testSetCode("C0")},
		{Code: "CB", ParentCode: testSetCode("A")},
		{Code: "CA", ParentCode: testSetCode("A")},
		{Code: "U", ParentCode: testSetCode("A")},
		{Code: "T1", ParentCode: testSetCode("A")},
		{Code: "T2", ParentCode: testSetCode("A")},
		{Code: "BC", ParentCode: testSetCode("B")},
	}
	hierarchy, err := newSetHierarchy(links)
	if err != nil {
		t.Fatal(err)
	}
	roots := []Set{{Code: "A", SetType: "expansion"}}
	wantCodes := []string{"C0", "CA", "CB", "GC", "T1", "T2", "U"}
	if codes := hierarchy.descendantCodes(roots); !reflect.DeepEqual(codes, wantCodes) {
		t.Fatalf("descendant codes = %#v, want %#v", codes, wantCodes)
	}
	descendants := []Set{
		{Code: "CB", ParentCode: testSetCode("A"), ReleasedAt: testSetDate(2021, 1, 1)},
		{Code: "T2", ParentCode: testSetCode("A"), SetType: "token"},
		{Code: "U", ParentCode: testSetCode("A")},
		{Code: "CA", ParentCode: testSetCode("A"), ReleasedAt: testSetDate(2021, 1, 1)},
		{Code: "T1", ParentCode: testSetCode("A"), SetType: "token", ReleasedAt: testSetDate(1990, 1, 1)},
		{Code: "GC", ParentCode: testSetCode("C0"), SetType: "token"},
		{Code: "C0", ParentCode: testSetCode("A"), ReleasedAt: testSetDate(2020, 1, 1)},
	}
	items, err := hierarchy.expand(roots, descendants)
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := []string{"A", "C0", "GC", "CA", "CB", "U", "T1", "T2"}
	wantDepth := []int{0, 1, 2, 1, 1, 1, 1, 1}
	if len(items) != len(wantOrder) {
		t.Fatalf("got %d items, want %d", len(items), len(wantOrder))
	}
	for i, item := range items {
		if item.Code != wantOrder[i] || item.Depth != wantDepth[i] {
			t.Errorf("item %d = %s depth %d, want %s depth %d", i, item.Code, item.Depth, wantOrder[i], wantDepth[i])
		}
	}
	if _, err := hierarchy.expand(roots, descendants[:len(descendants)-1]); err == nil {
		t.Error("incomplete subtree was accepted")
	}
}

func TestHierarchyRejectsMissingParentsAndCycles(t *testing.T) {
	tests := map[string][]hierarchyRow{
		"missing parent": {{Code: "A", ParentCode: testSetCode("missing")}},
		"self cycle":     {{Code: "A", ParentCode: testSetCode("A")}},
		"detached cycle": {{Code: "ROOT"}, {Code: "A", ParentCode: testSetCode("B")}, {Code: "B", ParentCode: testSetCode("A")}},
		"duplicate code": {{Code: "A"}, {Code: "a"}},
	}
	for name, rows := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := newSetHierarchy(rows); err == nil {
				t.Fatal("invalid hierarchy was accepted")
			}
		})
	}
}

func TestHierarchyKeepsSelectedRootOrder(t *testing.T) {
	hierarchy, err := newSetHierarchy([]hierarchyRow{{Code: "A"}, {Code: "B"}, {Code: "C", ParentCode: testSetCode("b")}})
	if err != nil {
		t.Fatal(err)
	}
	items, err := hierarchy.expand([]Set{{Code: "B"}, {Code: "A"}}, []Set{{Code: "C", ParentCode: testSetCode("b")}})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[0].Code != "B" || items[1].Code != "C" || items[2].Code != "A" {
		t.Fatalf("root order was changed: %#v", items)
	}
}
