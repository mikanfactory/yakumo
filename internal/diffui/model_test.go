package diffui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestEnterOpensZedOnChangesTab(t *testing.T) {
	var gotName string
	var gotArgs []string
	starter := func(name string, args ...string) error {
		gotName = name
		gotArgs = args
		return nil
	}

	m := Model{
		activeTab:     TabChanges,
		repoDir:       "/repo",
		editorStarter: starter,
		changes: ChangesModel{
			files:  []ChangedFile{{Path: "file.go"}},
			cursor: 0,
		},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a command, got nil")
	}

	result := cmd()
	msg, ok := result.(OpenEditorResultMsg)
	if !ok {
		t.Fatalf("expected OpenEditorResultMsg, got %T", result)
	}
	if msg.Err != nil {
		t.Errorf("expected no error, got %v", msg.Err)
	}

	if gotName != "zed" {
		t.Errorf("expected command %q, got %q", "zed", gotName)
	}
	if len(gotArgs) != 1 || gotArgs[0] != "/repo/file.go" {
		t.Errorf("expected args [/repo/file.go], got %v", gotArgs)
	}
}

func TestEnterPropagatesZedLaunchError(t *testing.T) {
	starter := func(name string, args ...string) error {
		return fmt.Errorf("not found")
	}

	m := Model{
		activeTab:     TabChanges,
		repoDir:       "/repo",
		editorStarter: starter,
		changes: ChangesModel{
			files:  []ChangedFile{{Path: "file.go"}},
			cursor: 0,
		},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a command, got nil")
	}

	result := cmd()
	msg, ok := result.(OpenEditorResultMsg)
	if !ok {
		t.Fatalf("expected OpenEditorResultMsg, got %T", result)
	}
	if msg.Err == nil {
		t.Fatal("expected an error from starter, got nil")
	}
}

func TestOpenEditorResultMsg_SetsStatusMsg(t *testing.T) {
	m := Model{}

	updated, _ := m.Update(OpenEditorResultMsg{Err: fmt.Errorf("test error")})
	model := updated.(Model)

	if model.statusMsg != "test error" {
		t.Errorf("expected statusMsg %q, got %q", "test error", model.statusMsg)
	}
}

func TestKeyPress_ClearsStatusMsg(t *testing.T) {
	m := Model{
		statusMsg: "some error",
		activeTab: TabChanges,
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := updated.(Model)

	if model.statusMsg != "" {
		t.Errorf("expected statusMsg to be cleared, got %q", model.statusMsg)
	}
}

func TestEnterOnEmptyFileList(t *testing.T) {
	m := Model{
		activeTab: TabChanges,
		changes:   ChangesModel{files: []ChangedFile{}},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected nil command when file list is empty")
	}
}

func TestEnterOnChecksTab(t *testing.T) {
	m := Model{
		activeTab: TabChecks,
		changes: ChangesModel{
			files: []ChangedFile{{Path: "file.go"}},
		},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected nil command when on Checks tab")
	}
}

func TestOKeyOpensPR_OnChecksTab(t *testing.T) {
	m := Model{
		activeTab: TabChecks,
		checks: ChecksModel{
			prURL: "https://github.com/owner/repo/pull/1",
		},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if cmd == nil {
		t.Fatal("expected a command for opening PR, got nil")
	}

	result := cmd()
	msg, ok := result.(OpenPRResultMsg)
	if !ok {
		t.Fatalf("expected OpenPRResultMsg, got %T", result)
	}
	// cmd.Start() will fail in test env (no browser), but the type is correct
	_ = msg
}

func TestOKeyNoop_WhenPRURLEmpty(t *testing.T) {
	m := Model{
		activeTab: TabChecks,
		checks: ChecksModel{
			prURL: "",
		},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if cmd != nil {
		t.Error("expected nil command when prURL is empty")
	}
}

func TestOKeyNoop_OnChangesTab(t *testing.T) {
	m := Model{
		activeTab: TabChanges,
		checks: ChecksModel{
			prURL: "https://github.com/owner/repo/pull/1",
		},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if cmd != nil {
		t.Error("expected nil command when on Changes tab")
	}
}
