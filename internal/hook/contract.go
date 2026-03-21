package hook

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// HookInput is the JSON received from AI client via stdin.
type HookInput struct {
	Hook       string          `json:"hook"`
	SessionID  string          `json:"session_id,omitempty"`
	ProjectDir string          `json:"project_dir,omitempty"`
	ProjectID  string          `json:"project_id,omitempty"`
	Timestamp  *time.Time      `json:"timestamp,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

// HookOutput is the JSON returned to AI client via stdout.
type HookOutput struct {
	ContextInjection string         `json:"context_injection,omitempty"`
	Status           string         `json:"status"` // "ok" | "skipped" | "error"
	Message          string         `json:"message,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

// SessionStartPayload is the payload for the session-start hook.
type SessionStartPayload struct {
	TaskDescription string `json:"task_description,omitempty"`
	InitialPrompt   string `json:"initial_prompt,omitempty"`
	WorkingDir      string `json:"working_dir,omitempty"`
}

// PromptSubmitPayload is the payload for the prompt-submit hook.
type PromptSubmitPayload struct {
	PromptText string `json:"prompt_text,omitempty"`
}

// SessionEndPayload is the payload for the session-end hook.
// Currently empty, reserved for future use.
type SessionEndPayload struct{}

// resolveSessionID returns the session ID from input if present,
// otherwise generates one from project_dir + pid + current time.
func resolveSessionID(input *HookInput) string {
	if input.SessionID != "" {
		return input.SessionID
	}
	raw := input.ProjectDir + strconv.Itoa(os.Getpid()) + time.Now().String()
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum)
}

// resolveProjectID returns the project ID from input if present,
// falls back to the base name of project_dir, or "unknown".
func resolveProjectID(input *HookInput) string {
	if input.ProjectID != "" {
		return input.ProjectID
	}
	if input.ProjectDir != "" {
		return filepath.Base(input.ProjectDir)
	}
	return "unknown"
}
