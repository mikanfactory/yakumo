package tmux

import (
	"fmt"
	"reflect"
	"testing"
)

func TestArchivingOptionName_IsStablePerPath(t *testing.T) {
	a := archivingOptionName("/code/repo-feat")
	if a != archivingOptionName("/code/repo-feat") {
		t.Error("expected the same name for the same path")
	}
	if a == archivingOptionName("/code/repo-other") {
		t.Error("expected different names for different paths")
	}
	if want := len(archivingOptionPrefix) + 12; len(a) != want {
		t.Errorf("expected name length %d, got %d (%s)", want, len(a), a)
	}
}

func TestMarkArchiving(t *testing.T) {
	name := archivingOptionName("/code/repo-feat")
	runner := &FakeRunner{
		Outputs: map[string]string{
			fmt.Sprintf("%v", []string{"set-option", "-g", name, "42:/code/repo-feat"}): "",
		},
	}

	if err := MarkArchiving(runner, "/code/repo-feat", 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarkArchiving_Error(t *testing.T) {
	runner := &FakeRunner{}

	if err := MarkArchiving(runner, "/code/repo-feat", 42); err == nil {
		t.Fatal("expected error")
	}
}

func TestUnmarkArchiving(t *testing.T) {
	name := archivingOptionName("/code/repo-feat")
	runner := &FakeRunner{
		Outputs: map[string]string{
			fmt.Sprintf("%v", []string{"set-option", "-gu", name}): "",
		},
	}

	if err := UnmarkArchiving(runner, "/code/repo-feat"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListArchiving(t *testing.T) {
	live := archivingOptionName("/code/repo-feat")
	spaced := archivingOptionName("/code/repo b")
	output := "@other value\n" +
		live + " 42:/code/repo-feat\n" +
		spaced + " \"43:/code/repo b\"\n" +
		"status on\n"
	runner := &FakeRunner{
		Outputs: map[string]string{"[show-options -g]": output},
	}

	paths, err := ListArchiving(runner, func(int) bool { return true })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"/code/repo-feat", "/code/repo b"}
	if !reflect.DeepEqual(paths, want) {
		t.Errorf("expected %v, got %v", want, paths)
	}
}

func TestListArchiving_DropsDeadProcesses(t *testing.T) {
	dead := archivingOptionName("/code/repo-dead")
	live := archivingOptionName("/code/repo-live")
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[show-options -g]": dead + " 7:/code/repo-dead\n" + live + " 8:/code/repo-live\n",
			fmt.Sprintf("%v", []string{"set-option", "-gu", dead}): "",
		},
	}

	paths, err := ListArchiving(runner, func(pid int) bool { return pid == 8 })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"/code/repo-live"}; !reflect.DeepEqual(paths, want) {
		t.Errorf("expected %v, got %v", want, paths)
	}

	unset := false
	for _, call := range runner.Calls {
		if reflect.DeepEqual(call, []string{"set-option", "-gu", dead}) {
			unset = true
		}
	}
	if !unset {
		t.Error("expected the dead mark to be removed")
	}
}

func TestListArchiving_IgnoresMalformedValues(t *testing.T) {
	name := archivingOptionName("/code/repo-feat")
	runner := &FakeRunner{
		Outputs: map[string]string{"[show-options -g]": name + " not-a-pid\n"},
	}

	paths, err := ListArchiving(runner, func(int) bool { return true })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected no paths, got %v", paths)
	}
}

func TestListArchiving_Error(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{"[show-options -g]": fmt.Errorf("no server")},
	}

	if _, err := ListArchiving(runner, func(int) bool { return true }); err == nil {
		t.Fatal("expected error")
	}
}
