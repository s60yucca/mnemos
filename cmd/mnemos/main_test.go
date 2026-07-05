package main

import "testing"

func TestConfigPathFromArgs(t *testing.T) {
	t.Setenv("MNEMOS_CONFIG", "/env/config.yaml")

	if got := configPathFromArgs(nil); got != "/env/config.yaml" {
		t.Fatalf("configPathFromArgs(nil) = %q, want /env/config.yaml", got)
	}
	if got := configPathFromArgs([]string{"--config", "/tmp/config.yaml", "status"}); got != "/tmp/config.yaml" {
		t.Fatalf("configPathFromArgs(--config) = %q, want /tmp/config.yaml", got)
	}
	if got := configPathFromArgs([]string{"--config=/tmp/inline.yaml", "status"}); got != "/tmp/inline.yaml" {
		t.Fatalf("configPathFromArgs(--config=) = %q, want /tmp/inline.yaml", got)
	}
}

func TestProjectIDFromArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "long flag space", args: []string{"--project", "alogame-workspace", "hook", "session-start"}, want: "alogame-workspace"},
		{name: "short flag space", args: []string{"-p", "hms", "serve"}, want: "hms"},
		{name: "long flag equals", args: []string{"--project=mnemos", "serve"}, want: "mnemos"},
		{name: "short flag equals", args: []string{"-p=mobile", "hook", "prompt-submit"}, want: "mobile"},
		{name: "missing", args: []string{"hook", "session-start"}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := projectIDFromArgs(tt.args); got != tt.want {
				t.Fatalf("projectIDFromArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSelectedCommandSkipsProjectFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "long flag space", args: []string{"--project", "alogame-workspace", "hook", "session-start"}, want: "hook"},
		{name: "short flag space", args: []string{"-p", "hms", "serve"}, want: "serve"},
		{name: "long flag equals", args: []string{"--project=mnemos", "check"}, want: "check"},
		{name: "short flag equals", args: []string{"-p=mobile", "doctor"}, want: "doctor"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectedCommand(tt.args); got != tt.want {
				t.Fatalf("selectedCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}
