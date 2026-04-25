package setup

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDeriveProjectID_UserProvided(t *testing.T) {
	result, err := DeriveProjectID("my-custom-project")
	if err != nil {
		t.Fatalf("DeriveProjectID failed: %v", err)
	}

	if result != "my-custom-project" {
		t.Errorf("expected 'my-custom-project', got %q", result)
	}
}

func TestDeriveProjectID_GitRepo(t *testing.T) {
	// Create a temporary git repo
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	// Initialize git repo
	if err := exec.Command("git", "init").Run(); err != nil {
		t.Skip("git not available")
	}

	// Set remote URL
	if err := exec.Command("git", "remote", "add", "origin", "https://github.com/user/test-repo.git").Run(); err != nil {
		t.Fatalf("failed to add remote: %v", err)
	}

	result, err := DeriveProjectID("")
	if err != nil {
		t.Fatalf("DeriveProjectID failed: %v", err)
	}

	if result != "test-repo" {
		t.Errorf("expected 'test-repo', got %q", result)
	}
}

func TestDeriveProjectID_NoGit(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "my-project-dir")
	if err := os.Mkdir(testDir, 0o755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(testDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	result, err := DeriveProjectID("")
	if err != nil {
		t.Fatalf("DeriveProjectID failed: %v", err)
	}

	if result != "my-project-dir" {
		t.Errorf("expected 'my-project-dir', got %q", result)
	}
}

func TestDeriveFromGit_HTTPS(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	if err := exec.Command("git", "init").Run(); err != nil {
		t.Skip("git not available")
	}

	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/user/repo.git", "repo"},
		{"https://github.com/user/repo", "repo"},
		{"git@github.com:user/repo.git", "repo"},
		{"git@github.com:user/repo", "repo"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			// Remove existing remote if any
			exec.Command("git", "remote", "remove", "origin").Run()

			if err := exec.Command("git", "remote", "add", "origin", tt.url).Run(); err != nil {
				t.Fatalf("failed to add remote: %v", err)
			}

			result, err := deriveFromGit()
			if err != nil {
				t.Fatalf("deriveFromGit failed: %v", err)
			}

			if result != tt.want {
				t.Errorf("deriveFromGit(%q) = %q, want %q", tt.url, result, tt.want)
			}
		})
	}
}

func TestSanitizeProjectID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"MyProject", "myproject"},
		{"my-project", "my-project"},
		{"my_project", "my-project"},
		{"my project", "my-project"},
		{"my@project!", "my-project"},
		{"my--project", "my--project"},
		{"-my-project-", "my-project"},
		{"123-project", "123-project"},
		{"project-123", "project-123"},
		{"my.project.name", "my-project-name"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeProjectID(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeProjectID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
