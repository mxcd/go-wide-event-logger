package wideevent

import "testing"

func TestNestSingleLevelKey(t *testing.T) {
	fields := []Field{{Key: "name", Value: "test"}}
	result := nestFields(fields)

	if result["name"] != "test" {
		t.Errorf("expected 'test', got %v", result["name"])
	}
}

func TestNestDotKey(t *testing.T) {
	fields := []Field{{Key: "a.b", Value: "val"}}
	result := nestFields(fields)

	a, ok := result["a"].(map[string]any)
	if !ok {
		t.Fatal("expected nested map for 'a'")
	}
	if a["b"] != "val" {
		t.Errorf("expected 'val', got %v", a["b"])
	}
}

func TestNestDeepKey(t *testing.T) {
	fields := []Field{{Key: "a.b.c", Value: "deep"}}
	result := nestFields(fields)

	a, ok := result["a"].(map[string]any)
	if !ok {
		t.Fatal("expected nested map for 'a'")
	}
	b, ok := a["b"].(map[string]any)
	if !ok {
		t.Fatal("expected nested map for 'b'")
	}
	if b["c"] != "deep" {
		t.Errorf("expected 'deep', got %v", b["c"])
	}
}

func TestNestMultipleKeysInSameGroup(t *testing.T) {
	fields := []Field{
		{Key: "a.b", Value: "one"},
		{Key: "a.c", Value: "two"},
	}
	result := nestFields(fields)

	a, ok := result["a"].(map[string]any)
	if !ok {
		t.Fatal("expected nested map for 'a'")
	}
	if a["b"] != "one" {
		t.Errorf("expected 'one', got %v", a["b"])
	}
	if a["c"] != "two" {
		t.Errorf("expected 'two', got %v", a["c"])
	}
}

func TestNestMixedFlatAndNested(t *testing.T) {
	fields := []Field{
		{Key: "flat", Value: "yes"},
		{Key: "nested.key", Value: "deep"},
	}
	result := nestFields(fields)

	if result["flat"] != "yes" {
		t.Errorf("expected 'yes', got %v", result["flat"])
	}
	nested, ok := result["nested"].(map[string]any)
	if !ok {
		t.Fatal("expected nested map for 'nested'")
	}
	if nested["key"] != "deep" {
		t.Errorf("expected 'deep', got %v", nested["key"])
	}
}

func TestNestFourLevels(t *testing.T) {
	fields := []Field{{Key: "a.b.c.d", Value: 42}}
	result := nestFields(fields)

	a := result["a"].(map[string]any)
	b := a["b"].(map[string]any)
	c := b["c"].(map[string]any)
	if c["d"] != 42 {
		t.Errorf("expected 42, got %v", c["d"])
	}
}
