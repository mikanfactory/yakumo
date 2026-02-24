package tui

import (
	"github.com/mikanfactory/yakumo/internal/model"
)

// NextSelectable returns the next selectable index after current, or current if none.
func NextSelectable(items []model.NavigableItem, current int) int {
	for i := current + 1; i < len(items); i++ {
		if items[i].Selectable {
			return i
		}
	}
	return current
}

// PrevSelectable returns the previous selectable index before current, or current if none.
func PrevSelectable(items []model.NavigableItem, current int) int {
	for i := current - 1; i >= 0; i-- {
		if items[i].Selectable {
			return i
		}
	}
	return current
}

// FirstSelectable returns the index of the first selectable item, or 0.
func FirstSelectable(items []model.NavigableItem) int {
	for i, item := range items {
		if item.Selectable {
			return i
		}
	}
	return 0
}
