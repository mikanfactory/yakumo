package tui

import (
	"testing"

	"github.com/mikanfactory/yakumo/internal/model"
)

func makeItems(selectables ...bool) []model.NavigableItem {
	items := make([]model.NavigableItem, len(selectables))
	for i, s := range selectables {
		items[i] = model.NavigableItem{Selectable: s}
	}
	return items
}

func TestNextSelectable(t *testing.T) {
	tests := []struct {
		name    string
		items   []model.NavigableItem
		current int
		want    int
	}{
		{"basic next", makeItems(true, false, true, true), 0, 2},
		{"skip non-selectable", makeItems(true, false, false, true), 0, 3},
		{"already at last", makeItems(true, false, true), 2, 2},
		{"no more selectable", makeItems(true, false, false), 0, 0},
		{"single item", makeItems(true), 0, 0},
		{"all non-selectable after", makeItems(true, false, false, false), 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NextSelectable(tt.items, tt.current)
			if got != tt.want {
				t.Errorf("NextSelectable(%d) = %d, want %d", tt.current, got, tt.want)
			}
		})
	}
}

func TestPrevSelectable(t *testing.T) {
	tests := []struct {
		name    string
		items   []model.NavigableItem
		current int
		want    int
	}{
		{"basic prev", makeItems(true, false, true, true), 3, 2},
		{"skip non-selectable", makeItems(true, false, false, true), 3, 0},
		{"already at first selectable", makeItems(false, true, true), 1, 1},
		{"no prev selectable", makeItems(false, false, true), 2, 2},
		{"single item", makeItems(true), 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PrevSelectable(tt.items, tt.current)
			if got != tt.want {
				t.Errorf("PrevSelectable(%d) = %d, want %d", tt.current, got, tt.want)
			}
		})
	}
}

func TestAdjustScroll(t *testing.T) {
	tests := []struct {
		name           string
		cursor         int
		viewportHeight int
		heights        []int
		want           int
	}{
		{
			name:           "empty heights returns 0",
			cursor:         0,
			viewportHeight: 10,
			heights:        []int{},
			want:           0,
		},
		{
			name:           "viewport unknown returns 0",
			cursor:         5,
			viewportHeight: 0,
			heights:        []int{1, 1, 1, 1, 1, 1},
			want:           0,
		},
		{
			name:           "all items fit in viewport",
			cursor:         3,
			viewportHeight: 10,
			heights:        []int{1, 1, 1, 1},
			want:           0,
		},
		{
			// heights[0..1] = 2 fits in vp=3, so scrollOff is pulled all the
			// way to 0 — the topmost item is shown above the cursor.
			name:           "cursor at top, viewport has room for items above",
			cursor:         1,
			viewportHeight: 3,
			heights:        []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			want:           0,
		},
		{
			// heights[3..5] = 3 fits exactly. heights[2..5] = 4 doesn't.
			name:           "cursor in middle, scrollOff reveals max items above",
			cursor:         5,
			viewportHeight: 3,
			heights:        []int{1, 1, 1, 1, 1, 1, 1, 1},
			want:           3,
		},
		{
			name:           "cursor near end, uniform heights",
			cursor:         9,
			viewportHeight: 3,
			heights:        []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			want:           7,
		},
		{
			// heights[3..8] = 1+2+1+1+1+2 = 8 fits exactly. heights[2..8] = 9 doesn't.
			// Old fixed-height algorithm (cursor - vp + 1) would return 1, but
			// heights[1..8] = 11 > 8, so 1 would push the cursor off-screen.
			name:           "action items at end inflate height and force more scroll",
			cursor:         8,
			viewportHeight: 8,
			heights:        []int{1, 1, 1, 1, 2, 1, 1, 1, 2},
			want:           3,
		},
		{
			// heights[8..10] = 4 ≤ 5. heights[7..10] = 6 > 5.
			// Old fixed-height algorithm: 10 - 5 + 1 = 6, but heights[6..10] = 7 > 5 — wrong.
			name:           "regression: fixed-height algorithm would be wrong here",
			cursor:         10,
			viewportHeight: 5,
			heights:        []int{1, 1, 2, 1, 2, 1, 1, 2, 1, 1, 2},
			want:           8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adjustScroll(tt.cursor, tt.viewportHeight, tt.heights)
			if got != tt.want {
				t.Errorf("adjustScroll(cursor=%d, vp=%d, heights=%v) = %d, want %d",
					tt.cursor, tt.viewportHeight, tt.heights, got, tt.want)
			}
		})
	}
}

func TestFirstSelectable(t *testing.T) {
	tests := []struct {
		name  string
		items []model.NavigableItem
		want  int
	}{
		{"first is selectable", makeItems(true, false, true), 0},
		{"first is header", makeItems(false, true, true), 1},
		{"all non-selectable", makeItems(false, false, false), 0},
		{"empty", makeItems(), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FirstSelectable(tt.items)
			if got != tt.want {
				t.Errorf("FirstSelectable() = %d, want %d", got, tt.want)
			}
		})
	}
}
