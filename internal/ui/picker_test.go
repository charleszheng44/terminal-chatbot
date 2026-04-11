package ui

import (
	"strings"
	"testing"
)

func TestNewModelPicker_PreSelectsCurrent(t *testing.T) {
	items := []string{"gpt-4o", "gpt-4o-mini", "claude-sonnet"}
	p := NewModelPicker(items, "gpt-4o-mini")
	if p.cursor != 1 {
		t.Errorf("cursor = %d, want 1", p.cursor)
	}
	if p.Selected() != "gpt-4o-mini" {
		t.Errorf("Selected = %q", p.Selected())
	}
}

func TestNewModelPicker_CurrentNotInList(t *testing.T) {
	items := []string{"a", "b", "c"}
	p := NewModelPicker(items, "z")
	if p.cursor != 0 {
		t.Errorf("cursor = %d, want 0", p.cursor)
	}
	if p.Selected() != "a" {
		t.Errorf("Selected = %q, want a", p.Selected())
	}
}

func TestNewModelPicker_EmptyList(t *testing.T) {
	p := NewModelPicker(nil, "anything")
	if p.cursor != 0 {
		t.Errorf("cursor = %d", p.cursor)
	}
	if p.Selected() != "" {
		t.Errorf("Selected on empty = %q", p.Selected())
	}
}

func TestModelPicker_MoveUpDown_Clamped(t *testing.T) {
	items := []string{"a", "b", "c"}
	p := NewModelPicker(items, "a")

	// MoveUp at top stays at 0.
	p.MoveUp()
	if p.cursor != 0 {
		t.Errorf("after MoveUp at top cursor = %d", p.cursor)
	}

	p.MoveDown()
	p.MoveDown()
	if p.cursor != 2 {
		t.Errorf("after 2x MoveDown cursor = %d", p.cursor)
	}

	// MoveDown at bottom stays at 2.
	p.MoveDown()
	if p.cursor != 2 {
		t.Errorf("after MoveDown at bottom cursor = %d", p.cursor)
	}

	p.MoveUp()
	if p.cursor != 1 {
		t.Errorf("after MoveUp cursor = %d", p.cursor)
	}
}

func TestModelPicker_MoveDown_EmptyList(t *testing.T) {
	p := NewModelPicker(nil, "")
	p.MoveDown()
	p.MoveUp()
	if p.cursor != 0 {
		t.Errorf("cursor = %d", p.cursor)
	}
}

func TestModelPicker_View_ContainsHeaderAndItems(t *testing.T) {
	items := []string{"alpha", "beta", "gamma"}
	p := NewModelPicker(items, "beta")
	out := p.View(80, 10)

	if !strings.Contains(out, "Select a model") {
		t.Errorf("view missing header:\n%s", out)
	}
	for _, it := range items {
		if !strings.Contains(out, it) {
			t.Errorf("view missing item %q:\n%s", it, out)
		}
	}
	if !strings.Contains(out, "(current)") {
		t.Errorf("view missing (current) marker:\n%s", out)
	}
	// The selected row carries the ❯ cursor glyph. It may be wrapped in
	// ANSI escape sequences, so look for the glyph anywhere.
	if !strings.Contains(out, "❯") {
		t.Errorf("view missing cursor glyph:\n%s", out)
	}
}

func TestModelPicker_View_CurrentMarkerAbsentWhenCurrentNotInList(t *testing.T) {
	p := NewModelPicker([]string{"a", "b"}, "z")
	out := p.View(40, 10)
	if strings.Contains(out, "(current)") {
		t.Errorf("unexpected (current) marker:\n%s", out)
	}
}

func TestModelPicker_View_ScrollsWithTallList(t *testing.T) {
	items := []string{"i0", "i1", "i2", "i3", "i4", "i5", "i6", "i7"}
	p := NewModelPicker(items, "i0")

	// Height is 5 → listHeight = 3. Move the cursor beyond what's visible
	// from offset 0 and verify the window scrolls.
	for i := 0; i < 5; i++ {
		p.MoveDown()
	}
	if p.cursor != 5 {
		t.Fatalf("cursor = %d, want 5", p.cursor)
	}

	out := p.View(40, 5)
	// i0 should have scrolled out of view; i5 should be visible.
	if strings.Contains(out, "i0") {
		t.Errorf("i0 should be scrolled off:\n%s", out)
	}
	if !strings.Contains(out, "i5") {
		t.Errorf("i5 should be visible:\n%s", out)
	}

	// Scrolling back up should bring early items back.
	for i := 0; i < 5; i++ {
		p.MoveUp()
	}
	out = p.View(40, 5)
	if !strings.Contains(out, "i0") {
		t.Errorf("i0 should be visible after scrolling up:\n%s", out)
	}
}

func TestModelPicker_View_EmptyList(t *testing.T) {
	p := NewModelPicker(nil, "")
	out := p.View(40, 10)
	if !strings.Contains(out, "Select a model") {
		t.Errorf("header missing on empty list view:\n%s", out)
	}
}

func TestModelPicker_View_TinyHeight(t *testing.T) {
	items := []string{"a", "b", "c"}
	p := NewModelPicker(items, "a")
	// Height 1 → listHeight clamps to 1; should render without panic.
	out := p.View(20, 1)
	if !strings.Contains(out, "Select a model") {
		t.Errorf("tiny-height view missing header:\n%s", out)
	}
}
