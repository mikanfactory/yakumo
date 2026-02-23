package tui

import (
	"testing"

	"github.com/mikanfactory/shiki/internal/model"
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
