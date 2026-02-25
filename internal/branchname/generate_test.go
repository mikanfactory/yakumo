package branchname

import (
	"errors"
	"testing"
)

func TestSanitizeBranchName_KebabCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"fix-login-redirect", "fix-login-redirect"},
		{"Fix Login Redirect", "fix-login-redirect"},
		{"ADD_USER_SETTINGS", "addusersettings"},
		{"simple", "simple"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SanitizeBranchName(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeBranchName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeBranchName_TruncatesAt30Chars(t *testing.T) {
	long := "this-is-a-very-long-branch-name-that-exceeds-thirty-characters"
	got := SanitizeBranchName(long)
	if len(got) > maxBranchNameLength {
		t.Errorf("len(SanitizeBranchName(%q)) = %d, want <= %d", long, len(got), maxBranchNameLength)
	}
}

func TestSanitizeBranchName_RemovesSpecialChars(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"fix: login bug!", "fix-login-bug"},
		{"feature/add-auth", "featureadd-auth"},
		{"hello@world#test", "helloworldtest"},
		{"  spaces  ", "spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SanitizeBranchName(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeBranchName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeBranchName_NoTrailingHyphen(t *testing.T) {
	// A name that's exactly 30 chars with a hyphen at position 30
	input := "a-very-long-branch-name-ending-"
	got := SanitizeBranchName(input)
	if got[len(got)-1] == '-' {
		t.Errorf("SanitizeBranchName(%q) = %q, should not end with hyphen", input, got)
	}
}

func TestSanitizeBranchName_EmptyInput(t *testing.T) {
	got := SanitizeBranchName("")
	if got != "" {
		t.Errorf("SanitizeBranchName(%q) = %q, want empty string", "", got)
	}
}

func TestFakeGenerator_Success(t *testing.T) {
	gen := FakeGenerator{Result: "fix-login", Err: nil}
	name, err := gen.GenerateBranchName("fix the login bug")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "fix-login" {
		t.Errorf("name = %q, want %q", name, "fix-login")
	}
}

func TestFakeGenerator_Error(t *testing.T) {
	gen := FakeGenerator{Err: errors.New("api error")}
	_, err := gen.GenerateBranchName("fix the login bug")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestSlugFromBranch(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"shoji/fix-login-redirect", "fix-login-redirect"},
		{"fix-login-redirect", "fix-login-redirect"},
		{"shoji/south-korea", "south-korea"},
		{"feature/add-auth/extra", "add-auth/extra"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SlugFromBranch(tt.input)
			if got != tt.want {
				t.Errorf("SlugFromBranch(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFilterEnv(t *testing.T) {
	env := []string{
		"PATH=/usr/bin",
		"CLAUDECODE=1",
		"HOME=/home/user",
		"CLAUDECODE_SESSION=abc",
	}

	filtered := filterEnv(env, "CLAUDECODE")
	for _, e := range filtered {
		if e == "CLAUDECODE=1" {
			t.Error("CLAUDECODE should have been filtered out")
		}
	}

	// CLAUDECODE_SESSION should NOT be filtered (different key)
	found := false
	for _, e := range filtered {
		if e == "CLAUDECODE_SESSION=abc" {
			found = true
		}
	}
	if !found {
		t.Error("CLAUDECODE_SESSION should not have been filtered")
	}

	if len(filtered) != 3 {
		t.Errorf("len(filtered) = %d, want 3", len(filtered))
	}
}
