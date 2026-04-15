//go:build benchmark

package benchmark

import (
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
)

// ScenarioCategory identifies the type of benchmark scenario.
type ScenarioCategory string

const (
	CategoryColdStartToWarm      ScenarioCategory = "COLD_START_TO_WARM"
	CategoryMistakePrevention    ScenarioCategory = "MISTAKE_PREVENTION"
	CategoryContextPrecision     ScenarioCategory = "CONTEXT_PRECISION"
	CategoryCrossSessionTransfer ScenarioCategory = "CROSS_SESSION_TRANSFER"
	CategoryCorrectionSupersedes ScenarioCategory = "CORRECTION_SUPERSEDES"
)

// BenchmarkScenario defines a complete simulation scenario.
type BenchmarkScenario struct {
	Name        string           `json:"name"`
	Category    ScenarioCategory `json:"category"`
	ProjectID   string           `json:"project_id"`
	Description string           `json:"description"`
	Tasks       []Task           `json:"tasks"`
	Sessions    int              `json:"sessions"`
	Memories    []SeedMemory     `json:"memories"`
}

// Task defines a single retrieval task within a scenario session.
type Task struct {
	Description       string   `json:"description"`
	Query             string   `json:"query"`
	RelevantMemoryIDs []string `json:"relevant_memory_ids"`
	GotchaMemoryIDs   []string `json:"gotcha_memory_ids"`
	CorrectionIDs     []string `json:"correction_ids"`
	TokenBudget       int      `json:"token_budget"`
}

// SeedMemory defines a memory to be seeded into the store at a given session.
// Ground truth (relevance, gotcha role) lives in Task, not here.
type SeedMemory struct {
	ID                   string            `json:"id"`
	Content              string            `json:"content"`
	Type                 domain.MemoryType `json:"type"`
	Category             string            `json:"category"`
	Tags                 []string          `json:"tags"`
	AvailableFromSession int               `json:"available_from_session"`
}

// SessionMetrics holds retrieval quality metrics for a single simulated session.
type SessionMetrics struct {
	SessionNumber          int     `json:"session_number"`
	TokensConsumed         int     `json:"tokens_consumed"`
	TotalStoreTokens       int     `json:"total_store_tokens"`
	TokenEfficiency        float64 `json:"token_efficiency"`
	ContextPrecision       float64 `json:"context_precision"`
	ContextRecall          float64 `json:"context_recall"`
	F1Score                float64 `json:"f1_score"`
	TruePositives          int     `json:"true_positives"`
	FalsePositives         int     `json:"false_positives"`
	FalseNegatives         int     `json:"false_negatives"`
	TotalGotchas           int     `json:"total_gotchas"`
	GotchasRetrieved       int     `json:"gotchas_retrieved"`
	CorrectionsRankedAbove int     `json:"corrections_ranked_above"`
}

// ScenarioResult aggregates session metrics for a completed scenario.
type ScenarioResult struct {
	Scenario              BenchmarkScenario `json:"scenario"`
	Sessions              []SessionMetrics  `json:"sessions"`
	SteadyStateSession    int               `json:"steady_state_session"`
	AvgPrecision          float64           `json:"avg_precision"`
	AvgRecall             float64           `json:"avg_recall"`
	AvgF1                 float64           `json:"avg_f1"`
	AvgTokenEfficiency    float64           `json:"avg_token_efficiency"`
	MistakePreventionRate float64           `json:"mistake_prevention_rate"`
	CorrectionRate        float64           `json:"correction_rate"`
	TransferRate          float64           `json:"transfer_rate"`
}

// BenchmarkReport is the top-level output of a benchmark run.
type BenchmarkReport struct {
	GeneratedAt time.Time        `json:"generated_at"`
	Results     []ScenarioResult `json:"results"`
	Summary     ReportSummary    `json:"summary"`
}

// ReportSummary provides a high-level overview across all scenarios.
type ReportSummary struct {
	BestF1Scenario         string  `json:"best_f1_scenario"`
	BestEfficiencyScenario string  `json:"best_efficiency_scenario"`
	OverallAvgF1           float64 `json:"overall_avg_f1"`
	OverallAvgEfficiency   float64 `json:"overall_avg_efficiency"`
}
