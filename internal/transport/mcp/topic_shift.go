package mcp

import (
	"github.com/mnemos-dev/mnemos/internal/hook"
	"github.com/mnemos-dev/mnemos/internal/observe"
)

const mcpTopicSimilarityThreshold = 0.4

func (s *Server) observeQueryTopicShift(projectID, query, source string) {
	topic := hook.DetectTopic(query)
	if topic == "" {
		return
	}

	key := projectID
	if key == "" {
		key = "__global__"
	}

	s.topicMu.Lock()
	defer s.topicMu.Unlock()

	previous := s.activeTopics[key]
	if previous != "" && hook.TopicChanged(topic, previous, mcpTopicSimilarityThreshold) {
		observe.Feature("topic_shift", map[string]any{
			"changed":    true,
			"topic":      topic,
			"previous":   previous,
			"project_id": projectID,
			"source":     source,
		})
	}
	s.activeTopics[key] = topic
}
