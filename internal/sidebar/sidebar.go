package sidebar

import (
	"worktree-ui/internal/model"
)

// BuildItems converts RepoGroups into a flat NavigableItem list
// suitable for the TUI model's cursor navigation.
func BuildItems(groups []model.RepoGroup) []model.NavigableItem {
	var items []model.NavigableItem

	for _, group := range groups {
		items = append(items, model.NavigableItem{
			Kind:       model.ItemKindGroupHeader,
			Label:      group.Name,
			Selectable: false,
		})

		for _, wt := range group.Worktrees {
			items = append(items, model.NavigableItem{
				Kind:         model.ItemKindWorktree,
				Label:        wt.Branch,
				Selectable:   true,
				WorktreePath: wt.Path,
				RepoRootPath: group.RootPath,
				Status:       wt.Status,
				IsBare:       wt.IsBare,
			})
		}

		items = append(items, model.NavigableItem{
			Kind:         model.ItemKindAddWorktree,
			Label:        "+ Add worktree",
			Selectable:   true,
			RepoRootPath: group.RootPath,
		})
	}

	items = append(items,
		model.NavigableItem{
			Kind:       model.ItemKindAddRepo,
			Label:      "+ Add repository",
			Selectable: true,
		},
		model.NavigableItem{
			Kind:       model.ItemKindSettings,
			Label:      "Settings",
			Selectable: true,
		},
	)

	return items
}
