package setlist

import (
	"fmt"
	"sort"
	"strings"
)

type hierarchyRow struct {
	Code       string  `db:"code"`
	ParentCode *string `db:"parent_code"`
}

type setHierarchy struct {
	nodes    map[string]hierarchyRow
	children map[string][]string
}

func newSetHierarchy(rows []hierarchyRow) (*setHierarchy, error) {
	hierarchy := &setHierarchy{
		nodes:    make(map[string]hierarchyRow, len(rows)),
		children: make(map[string][]string),
	}
	for _, row := range rows {
		key := strings.ToLower(row.Code)
		if key == "" {
			return nil, fmt.Errorf("set code is empty")
		}
		if _, exists := hierarchy.nodes[key]; exists {
			return nil, fmt.Errorf("duplicate set code %q", row.Code)
		}
		hierarchy.nodes[key] = row
	}
	for _, row := range rows {
		if row.ParentCode == nil {
			continue
		}
		parent := strings.ToLower(*row.ParentCode)
		if _, exists := hierarchy.nodes[parent]; !exists {
			return nil, fmt.Errorf("set %q has missing parent %q", row.Code, *row.ParentCode)
		}
		hierarchy.children[parent] = append(hierarchy.children[parent], strings.ToLower(row.Code))
	}
	state := make(map[string]uint8, len(rows))
	var visit func(string) error
	visit = func(key string) error {
		if state[key] == 1 {
			return fmt.Errorf("set hierarchy contains a cycle at %q", hierarchy.nodes[key].Code)
		}
		if state[key] == 2 {
			return nil
		}
		state[key] = 1
		if parent := hierarchy.nodes[key].ParentCode; parent != nil {
			if err := visit(strings.ToLower(*parent)); err != nil {
				return err
			}
		}
		state[key] = 2
		return nil
	}
	for key := range hierarchy.nodes {
		if err := visit(key); err != nil {
			return nil, err
		}
	}
	return hierarchy, nil
}

func (h *setHierarchy) descendantCodes(roots []Set) []string {
	codes := make([]string, 0)
	var visit func(string)
	visit = func(key string) {
		for _, child := range h.children[key] {
			codes = append(codes, h.nodes[child].Code)
			visit(child)
		}
	}
	for _, root := range roots {
		visit(strings.ToLower(root.Code))
	}
	sort.Strings(codes)
	return codes
}

func (h *setHierarchy) expand(roots, descendants []Set) ([]Set, error) {
	items := make(map[string]Set, len(roots)+len(descendants))
	for _, group := range [][]Set{roots, descendants} {
		for _, item := range group {
			key := strings.ToLower(item.Code)
			if _, exists := items[key]; exists {
				return nil, fmt.Errorf("duplicate set result %q", item.Code)
			}
			items[key] = item
		}
	}
	result := make([]Set, 0, len(items))
	var visit func(Set, int) error
	visit = func(item Set, depth int) error {
		item.Depth = depth
		result = append(result, item)
		children := make([]Set, 0, len(h.children[strings.ToLower(item.Code)]))
		for _, key := range h.children[strings.ToLower(item.Code)] {
			child, ok := items[key]
			if !ok {
				return fmt.Errorf("set subtree is missing %q", h.nodes[key].Code)
			}
			children = append(children, child)
		}
		sort.Slice(children, func(i, j int) bool {
			return childSetLess(children[i], children[j])
		})
		for _, child := range children {
			if err := visit(child, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	for _, root := range roots {
		if err := visit(root, 0); err != nil {
			return nil, err
		}
	}
	if len(result) != len(items) {
		return nil, fmt.Errorf("set results contain records outside the selected root groups")
	}
	return result, nil
}

func childSetLess(left, right Set) bool {
	leftToken := left.SetType == "token"
	rightToken := right.SetType == "token"
	if leftToken != rightToken {
		return !leftToken
	}
	if left.ReleasedAt == nil || right.ReleasedAt == nil {
		if (left.ReleasedAt == nil) != (right.ReleasedAt == nil) {
			return left.ReleasedAt != nil
		}
	} else if !left.ReleasedAt.Equal(*right.ReleasedAt) {
		return left.ReleasedAt.Before(*right.ReleasedAt)
	}
	return strings.ToLower(left.Code) < strings.ToLower(right.Code)
}
