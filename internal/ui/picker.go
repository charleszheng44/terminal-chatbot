package ui

import (
	"strings"
)

// ModelPicker is a minimal list picker used by /model. It renders a flat
// list of models, tracks a cursor, and scrolls to keep the cursor in view
// when the list is taller than the available height.
type ModelPicker struct {
	items   []string
	cursor  int
	current string // the currently-active model, shown with a "(current)" marker
	offset  int    // index of the first visible item (scroll window start)
}

// NewModelPicker builds a picker over the given items, pre-selecting the
// current model if it appears in the list.
func NewModelPicker(items []string, current string) *ModelPicker {
	p := &ModelPicker{
		items:   items,
		current: current,
	}
	for i, it := range items {
		if it == current {
			p.cursor = i
			break
		}
	}
	return p
}

// MoveUp moves the cursor up by one, stopping at the top.
func (p *ModelPicker) MoveUp() {
	if p.cursor > 0 {
		p.cursor--
	}
}

// MoveDown moves the cursor down by one, stopping at the bottom.
func (p *ModelPicker) MoveDown() {
	if p.cursor < len(p.items)-1 {
		p.cursor++
	}
}

// Selected returns the item under the cursor, or "" if the list is empty.
func (p *ModelPicker) Selected() string {
	if len(p.items) == 0 {
		return ""
	}
	return p.items[p.cursor]
}

// View renders the picker within the given width and height. The height
// includes the header line and the blank line beneath it.
func (p *ModelPicker) View(width, height int) string {
	header := systemMsgStyle.Render(
		"Select a model — ↑/↓ to navigate · Enter to switch · Esc to cancel",
	)

	// Reserve two rows for the header and the blank line under it.
	listHeight := height - 2
	if listHeight < 1 {
		listHeight = 1
	}

	p.updateScrollWindow(listHeight)

	end := p.offset + listHeight
	if end > len(p.items) {
		end = len(p.items)
	}

	var rows []string
	for i := p.offset; i < end; i++ {
		rows = append(rows, p.renderRow(i))
	}

	// Pad to full listHeight so the input box sits at a stable position.
	for len(rows) < listHeight {
		rows = append(rows, "")
	}

	body := strings.Join(rows, "\n")
	_ = width // reserved for future truncation/wrapping; not needed yet
	return header + "\n\n" + body
}

// renderRow renders a single row with the cursor prefix and current marker.
func (p *ModelPicker) renderRow(i int) string {
	item := p.items[i]
	var prefix string
	if i == p.cursor {
		prefix = inputPromptStyle.Render("❯ ")
	} else {
		prefix = "  "
	}

	line := prefix + item
	if item == p.current {
		line += "  " + systemMsgStyle.Render("(current)")
	}
	return line
}

// updateScrollWindow slides the visible window so the cursor stays inside
// it. Called on every View so resizes are handled automatically.
func (p *ModelPicker) updateScrollWindow(listHeight int) {
	if p.cursor < p.offset {
		p.offset = p.cursor
	}
	if p.cursor >= p.offset+listHeight {
		p.offset = p.cursor - listHeight + 1
	}
	if p.offset < 0 {
		p.offset = 0
	}
	maxOffset := len(p.items) - listHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if p.offset > maxOffset {
		p.offset = maxOffset
	}
}
