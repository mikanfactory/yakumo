package tui

import (
	"fmt"
	"log"
	"slices"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mikanfactory/yakumo/internal/model"
	"github.com/mikanfactory/yakumo/internal/sidebar"
)

// updateBackground handles results of background work regardless of the
// current modal so that completions are never swallowed by an open dialog.
func (m Model) updateBackground(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case GitDataMsg:
		if msg.Seq < m.refreshSeq {
			return m, nil, true
		}
		m = m.applyGroups(msg.Groups)
		m.loading = false
		if !m.agentTickRunning {
			m.agentTickRunning = true
			return m, agentTickCmd(), true
		}
		return m, nil, true

	case GitDataErrMsg:
		m.err = msg.Err
		m.loading = false
		return m, nil, true

	case AgentTickMsg:
		if len(m.groups) > 0 && m.tmuxRunner != nil {
			return m, fetchAgentStatusCmd(m.tmuxRunner, m.runner, m.groups), true
		}
		return m, agentTickCmd(), true

	case AgentStatusMsg:
		m.agentStatus = msg.Statuses
		m.items = applyAgentStatus(m.items, m.agentStatus)
		return m, agentTickCmd(), true

	case WorktreeArchivedMsg:
		m = m.clearPending(msg.WorktreePath)
		m = m.applyGroups(removeWorktree(m.groups, msg.WorktreePath))
		if m.quitWhenIdle && len(m.pendingOps) == 0 {
			next, cmd := m.quitIfIdle()
			return next, cmd, true
		}
		next, cmd := m.refresh()
		return next, cmd, true

	case WorktreeArchiveErrMsg:
		m = m.clearPending(msg.WorktreePath)
		m.err = msg.Err
		log.Printf("[archive] %s: %v", msg.WorktreePath, msg.Err)
		next, cmd := m.quitIfIdle()
		return next, cmd, true

	case SessionReadyMsg:
		m = m.clearPending(msg.WorktreePath)
		m.selected = msg.WorktreePath
		m.selectedRepoPath = msg.RepoRootPath
		m.quitWhenIdle = true
		next, cmd := m.quitIfIdle()
		return next, cmd, true

	case SessionSetupErrMsg:
		m = m.clearPending(msg.WorktreePath)
		m.err = msg.Err
		log.Printf("[setup] %s: %v", msg.WorktreePath, msg.Err)
		next, cmd := m.quitIfIdle()
		return next, cmd, true
	}

	return m, nil, false
}

func (m Model) selectWorktree(item model.NavigableItem) (tea.Model, tea.Cmd) {
	if m.sessionSetup == nil {
		m.selected = item.WorktreePath
		m.selectedRepoPath = item.RepoRootPath
		return m, tea.Quit
	}
	if m.isPending(item.WorktreePath) || m.hasPending(opSetup) {
		return m, nil
	}
	m = m.markPending(item.WorktreePath, opSetup)
	return m, sessionSetupCmd(m.sessionSetup, item.WorktreePath, item.RepoRootPath)
}

func sessionSetupCmd(setup SessionSetupFunc, worktreePath, repoRootPath string) tea.Cmd {
	return func() tea.Msg {
		if err := setup(worktreePath, repoRootPath); err != nil {
			return SessionSetupErrMsg{WorktreePath: worktreePath, Err: err}
		}
		return SessionReadyMsg{WorktreePath: worktreePath, RepoRootPath: repoRootPath}
	}
}

// requestQuit defers quitting while background work is running, because
// exiting the program would kill those goroutines mid-operation. A forced
// request while already waiting quits immediately.
func (m Model) requestQuit(force bool) (tea.Model, tea.Cmd) {
	if len(m.pendingOps) == 0 || (force && m.quitWhenIdle) {
		m.quitting = true
		return m, tea.Quit
	}
	m.quitWhenIdle = true
	return m, nil
}

func (m Model) quitIfIdle() (Model, tea.Cmd) {
	if !m.quitWhenIdle || len(m.pendingOps) > 0 {
		return m, nil
	}
	if m.selected == "" {
		m.quitting = true
	}
	return m, tea.Quit
}

// refresh re-fetches git data without blocking the UI. Responses from
// superseded refreshes are discarded via Seq.
func (m Model) refresh() (Model, tea.Cmd) {
	m.refreshSeq++
	seq := m.refreshSeq
	fetch := fetchGitDataCmd(m.config, m.runner)
	return m, func() tea.Msg {
		msg := fetch()
		if data, ok := msg.(GitDataMsg); ok {
			data.Seq = seq
			return data
		}
		return msg
	}
}

func (m Model) isPending(worktreePath string) bool {
	_, ok := m.pendingOps[worktreePath]
	return ok
}

func (m Model) hasPending(op pendingOp) bool {
	for _, o := range m.pendingOps {
		if o == op {
			return true
		}
	}
	return false
}

func (m Model) markPending(worktreePath string, op pendingOp) Model {
	ops := make(map[string]pendingOp, len(m.pendingOps)+1)
	for k, v := range m.pendingOps {
		ops[k] = v
	}
	ops[worktreePath] = op
	m.pendingOps = ops
	return m
}

func (m Model) clearPending(worktreePath string) Model {
	if !m.isPending(worktreePath) {
		return m
	}
	ops := make(map[string]pendingOp, len(m.pendingOps))
	for k, v := range m.pendingOps {
		if k != worktreePath {
			ops[k] = v
		}
	}
	m.pendingOps = ops
	return m
}

func (m Model) pendingLabel(item model.NavigableItem) string {
	if item.Kind != model.ItemKindWorktree {
		return ""
	}
	switch m.pendingOps[item.WorktreePath] {
	case opSetup:
		return "setting up…"
	case opArchive:
		return "archiving…"
	}
	return ""
}

func (m Model) pendingSummary() string {
	n := len(m.pendingOps)
	suffix := "s"
	if n == 1 {
		suffix = ""
	}
	return fmt.Sprintf("waiting for %d operation%s…  ctrl+c: force quit", n, suffix)
}

// applyGroups rebuilds items from groups, keeping the cursor on the same item
// when it still exists.
func (m Model) applyGroups(groups []model.RepoGroup) Model {
	var prev *model.NavigableItem
	if m.cursor >= 0 && m.cursor < len(m.items) {
		p := m.items[m.cursor]
		prev = &p
	}
	m.groups = groups
	m.items = applyAgentStatus(sidebar.BuildItems(groups), m.agentStatus)
	m.cursor = restoreCursor(m.items, prev, m.cursor)
	return recomputeScroll(m)
}

func applyAgentStatus(items []model.NavigableItem, statuses map[string][]model.AgentInfo) []model.NavigableItem {
	for i := range items {
		if items[i].Kind == model.ItemKindWorktree {
			items[i].AgentStatus = statuses[items[i].WorktreePath]
		}
	}
	return items
}

func restoreCursor(items []model.NavigableItem, prev *model.NavigableItem, prevCursor int) int {
	if len(items) == 0 {
		return 0
	}
	if prev == nil {
		return FirstSelectable(items)
	}
	for i, it := range items {
		if it.Selectable && it.Kind == prev.Kind && it.WorktreePath == prev.WorktreePath && it.RepoRootPath == prev.RepoRootPath {
			return i
		}
	}
	idx := min(prevCursor, len(items)-1)
	if items[idx].Selectable {
		return idx
	}
	if next := NextSelectable(items, idx); next != idx {
		return next
	}
	if p := PrevSelectable(items, idx); p != idx {
		return p
	}
	return FirstSelectable(items)
}

func removeWorktree(groups []model.RepoGroup, worktreePath string) []model.RepoGroup {
	out := make([]model.RepoGroup, len(groups))
	for i, g := range groups {
		g.Worktrees = slices.DeleteFunc(slices.Clone(g.Worktrees), func(w model.WorktreeInfo) bool {
			return w.Path == worktreePath
		})
		out[i] = g
	}
	return out
}
