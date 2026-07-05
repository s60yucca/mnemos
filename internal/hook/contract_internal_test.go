package hook

import "testing"

func TestResolveProjectIDPrefersExplicitInput(t *testing.T) {
	t.Setenv("MNEMOS_PROJECT_ID", "env-project")

	got := resolveProjectID(&HookInput{
		ProjectID:  "explicit-project",
		ProjectDir: "/tmp/cwd-project",
	})

	if got != "explicit-project" {
		t.Fatalf("resolveProjectID() = %q, want explicit-project", got)
	}
}

func TestResolveProjectIDUsesEnvBeforeCwd(t *testing.T) {
	t.Setenv("MNEMOS_PROJECT_ID", "env-project")

	got := resolveProjectID(&HookInput{ProjectDir: "/tmp/cwd-project"})

	if got != "env-project" {
		t.Fatalf("resolveProjectID() = %q, want env-project", got)
	}
}
