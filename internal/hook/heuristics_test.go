package hook

import "testing"

func TestIsGenericPrompt(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"continue", true},
		{"Continue.", true}, // normalized
		{"ok", true},
		{"okay", true},
		{"yes", true},
		{"no", true},
		{"sure", true},
		{"thanks", true},
		{"thank you", true},
		{"go ahead", true},
		{"proceed", true},
		{"next", true},
		{"keep going", true},
		{"looks good", true},
		{"lgtm", true},
		{"do it", true},
		{"go on", true},
		{"right", true},
		{"yep", true},
		{"yeah", true},
		{"nah", true},
		{"nope", true},
		{"fine", true},
		{"great", true},
		{"perfect", true},
		{"sounds good", true},
		{"makes sense", true},
		{"got it", true},
		{"understood", true},
		{"yes please", false}, // "please" not in generic set
		{"fix the auth bug", false},
		{"ok let's refactor", false}, // "ok" matches but full string does not
		{"implement authentication", false},
		{"", false},
	}

	for _, c := range cases {
		got := IsGenericPrompt(c.input)
		if got != c.want {
			t.Errorf("IsGenericPrompt(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestDetectTopic(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"fix the auth middleware bug", "fix auth middleware bug"},
		{"what's the best caching strategy for API?", "whats best caching strategy api"},
		{"continue", "continue"},
		{"", ""},
		{"the a an is are", ""},
		{"implement JWT authentication middleware", "implement jwt authentication middleware"},
	}

	for _, c := range cases {
		got := DetectTopic(c.input)
		if got != c.want {
			t.Errorf("DetectTopic(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestTopicChanged(t *testing.T) {
	const threshold = 0.3

	cases := []struct {
		newTopic    string
		activeTopic string
		want        bool
	}{
		// intersection=2, union=4, similarity=0.5 → NOT changed
		{"auth middleware bug", "auth middleware fix", false},
		// intersection=0, union=4, similarity=0.0 → CHANGED
		{"auth middleware", "database pooling", true},
		// intersection=1, union=5, similarity=0.2 → CHANGED
		{"api gateway caching", "api rate limiting", true},
		// both empty → changed
		{"", "", true},
		// identical → not changed
		{"auth middleware", "auth middleware", false},
	}

	for _, c := range cases {
		got := TopicChanged(c.newTopic, c.activeTopic, threshold)
		if got != c.want {
			t.Errorf("TopicChanged(%q, %q, %.1f) = %v, want %v",
				c.newTopic, c.activeTopic, threshold, got, c.want)
		}
	}
}
