package benchmark

import "github.com/mnemos-dev/mnemos/internal/domain"

// BuiltinScenarios returns the five built-in benchmark scenarios, one per category.
func BuiltinScenarios() []BenchmarkScenario {
	return []BenchmarkScenario{
		coldStartToWarm(),
		mistakePrevention(),
		contextPrecision(),
		crossSessionTransfer(),
		correctionSupersedes(),
	}
}

// coldStartToWarm simulates a growing CLI-tool knowledge base across 20 sessions.
// Memories become available at staggered sessions (1–15), measuring how F1 evolves
// as the store accumulates.
func coldStartToWarm() BenchmarkScenario {
	memories := []SeedMemory{
		{ID: "cold-mem-1", Content: "Use 'git stash pop' to restore stashed changes; 'git stash apply' keeps the stash entry.", Type: domain.MemoryTypeLongTerm, Category: "cli", Tags: []string{"git", "stash"}, AvailableFromSession: 1},
		{ID: "cold-mem-2", Content: "The '--dry-run' flag in rsync previews file transfers without making changes.", Type: domain.MemoryTypeLongTerm, Category: "cli", Tags: []string{"rsync", "flags"}, AvailableFromSession: 2},
		{ID: "cold-mem-3", Content: "Use 'jq -r' to output raw strings without JSON quoting in shell pipelines.", Type: domain.MemoryTypeLongTerm, Category: "cli", Tags: []string{"jq", "json"}, AvailableFromSession: 3},
		{ID: "cold-mem-4", Content: "The 'xargs -P' flag enables parallel execution of commands from stdin.", Type: domain.MemoryTypeLongTerm, Category: "cli", Tags: []string{"xargs", "parallel"}, AvailableFromSession: 4},
		{ID: "cold-mem-5", Content: "Use 'awk NR==FNR' pattern to join two files on a common field without sorting.", Type: domain.MemoryTypeLongTerm, Category: "cli", Tags: []string{"awk", "join"}, AvailableFromSession: 5},
		{ID: "cold-mem-6", Content: "The 'find -exec {} +' form batches arguments, unlike '-exec {} \\;' which forks per file.", Type: domain.MemoryTypeLongTerm, Category: "cli", Tags: []string{"find", "exec"}, AvailableFromSession: 6},
		{ID: "cold-mem-7", Content: "Use 'curl -o /dev/null -w \"%{http_code}\"' to check HTTP status codes silently.", Type: domain.MemoryTypeLongTerm, Category: "cli", Tags: []string{"curl", "http"}, AvailableFromSession: 7},
		{ID: "cold-mem-8", Content: "The 'sed -i.bak' flag creates a backup before in-place editing on macOS.", Type: domain.MemoryTypeLongTerm, Category: "cli", Tags: []string{"sed", "edit"}, AvailableFromSession: 8},
		{ID: "cold-mem-9", Content: "Use 'grep -l' to list only filenames containing a pattern, not the matching lines.", Type: domain.MemoryTypeLongTerm, Category: "cli", Tags: []string{"grep", "search"}, AvailableFromSession: 9},
		{ID: "cold-mem-10", Content: "The 'sort -k2,2n' flag sorts numerically by the second field in tab-delimited files.", Type: domain.MemoryTypeLongTerm, Category: "cli", Tags: []string{"sort", "fields"}, AvailableFromSession: 10},
		{ID: "cold-mem-11", Content: "Use 'tee' to write stdout to a file while still piping it to the next command.", Type: domain.MemoryTypeLongTerm, Category: "cli", Tags: []string{"tee", "pipe"}, AvailableFromSession: 11},
		{ID: "cold-mem-12", Content: "The 'watch -d' flag highlights differences between successive command outputs.", Type: domain.MemoryTypeLongTerm, Category: "cli", Tags: []string{"watch", "diff"}, AvailableFromSession: 12},
		{ID: "cold-mem-13", Content: "Use 'lsof -i :8080' to identify which process is listening on a specific port.", Type: domain.MemoryTypeLongTerm, Category: "cli", Tags: []string{"lsof", "network"}, AvailableFromSession: 13},
		{ID: "cold-mem-14", Content: "The 'tr -s' flag squeezes repeated characters into a single occurrence.", Type: domain.MemoryTypeLongTerm, Category: "cli", Tags: []string{"tr", "text"}, AvailableFromSession: 14},
		{ID: "cold-mem-15", Content: "Use 'cut -d: -f1' to extract the first colon-delimited field from each line.", Type: domain.MemoryTypeLongTerm, Category: "cli", Tags: []string{"cut", "fields"}, AvailableFromSession: 15},
	}

	tasks := []Task{
		{
			Description:       "Query git stash usage",
			Query:             "git stash restore changes",
			RelevantMemoryIDs: []string{"cold-mem-1"},
			TokenBudget:       2000,
		},
		{
			Description:       "Query JSON processing in shell",
			Query:             "jq raw string output shell pipeline",
			RelevantMemoryIDs: []string{"cold-mem-3"},
			TokenBudget:       2000,
		},
		{
			Description:       "Query parallel command execution",
			Query:             "parallel execution xargs commands",
			RelevantMemoryIDs: []string{"cold-mem-4"},
			TokenBudget:       2000,
		},
		{
			Description:       "Query HTTP status checking",
			Query:             "curl check http status code silent",
			RelevantMemoryIDs: []string{"cold-mem-7"},
			TokenBudget:       2000,
		},
		{
			Description:       "Query process port listening",
			Query:             "find process listening on port",
			RelevantMemoryIDs: []string{"cold-mem-13"},
			TokenBudget:       2000,
		},
	}

	return BenchmarkScenario{
		Name:        "cold-start-to-warm",
		Category:    CategoryColdStartToWarm,
		ProjectID:   "benchmark",
		Description: "Measures how retrieval F1 improves as CLI tool memories accumulate across 20 sessions.",
		Sessions:    20,
		Tasks:       tasks,
		Memories:    memories,
	}
}

// mistakePrevention seeds 10 gotcha memories (wrong approaches) and 5 relevant memories,
// measuring whether the retrieval engine avoids surfacing the wrong-approach memories.
func mistakePrevention() BenchmarkScenario {
	memories := []SeedMemory{
		// 10 gotcha memories — wrong approaches
		{ID: "mistake-gotcha-1", Content: "Use 'git push --force' to overwrite remote history when branches diverge.", Type: domain.MemoryTypeEpisodic, Category: "gotcha", Tags: []string{"git", "dangerous"}, AvailableFromSession: 1},
		{ID: "mistake-gotcha-2", Content: "Parse JSON with regex in bash using grep and sed for quick extraction.", Type: domain.MemoryTypeEpisodic, Category: "gotcha", Tags: []string{"json", "regex"}, AvailableFromSession: 1},
		{ID: "mistake-gotcha-3", Content: "Store database passwords directly in environment variables in .bashrc.", Type: domain.MemoryTypeEpisodic, Category: "gotcha", Tags: []string{"security", "passwords"}, AvailableFromSession: 1},
		{ID: "mistake-gotcha-4", Content: "Use 'rm -rf /' with sudo to clean up disk space on Linux systems.", Type: domain.MemoryTypeEpisodic, Category: "gotcha", Tags: []string{"dangerous", "rm"}, AvailableFromSession: 1},
		{ID: "mistake-gotcha-5", Content: "Disable SSL certificate verification with curl -k for faster API calls.", Type: domain.MemoryTypeEpisodic, Category: "gotcha", Tags: []string{"security", "curl"}, AvailableFromSession: 1},
		{ID: "mistake-gotcha-6", Content: "Use eval() to parse JSON in JavaScript for simplicity.", Type: domain.MemoryTypeEpisodic, Category: "gotcha", Tags: []string{"javascript", "security"}, AvailableFromSession: 1},
		{ID: "mistake-gotcha-7", Content: "Commit API keys and tokens directly to the repository for convenience.", Type: domain.MemoryTypeEpisodic, Category: "gotcha", Tags: []string{"security", "secrets"}, AvailableFromSession: 1},
		{ID: "mistake-gotcha-8", Content: "Use SELECT * in production queries to avoid specifying column names.", Type: domain.MemoryTypeEpisodic, Category: "gotcha", Tags: []string{"sql", "performance"}, AvailableFromSession: 1},
		{ID: "mistake-gotcha-9", Content: "Catch all exceptions with a bare 'except:' clause in Python to prevent crashes.", Type: domain.MemoryTypeEpisodic, Category: "gotcha", Tags: []string{"python", "exceptions"}, AvailableFromSession: 1},
		{ID: "mistake-gotcha-10", Content: "Use mutable default arguments in Python functions for shared state.", Type: domain.MemoryTypeEpisodic, Category: "gotcha", Tags: []string{"python", "mutable"}, AvailableFromSession: 1},
		// 5 relevant memories — correct approaches
		{ID: "mistake-rel-1", Content: "Use 'git push --force-with-lease' instead of --force to prevent overwriting others' work.", Type: domain.MemoryTypeLongTerm, Category: "best-practice", Tags: []string{"git", "safe"}, AvailableFromSession: 1},
		{ID: "mistake-rel-2", Content: "Use 'jq' for JSON parsing in shell scripts — it handles edge cases regex cannot.", Type: domain.MemoryTypeLongTerm, Category: "best-practice", Tags: []string{"json", "jq"}, AvailableFromSession: 1},
		{ID: "mistake-rel-3", Content: "Store secrets in a secrets manager (Vault, AWS SSM) or .env files excluded from git.", Type: domain.MemoryTypeLongTerm, Category: "best-practice", Tags: []string{"security", "secrets"}, AvailableFromSession: 1},
		{ID: "mistake-rel-4", Content: "Always verify SSL certificates; use --cacert to specify a custom CA bundle if needed.", Type: domain.MemoryTypeLongTerm, Category: "best-practice", Tags: []string{"security", "ssl"}, AvailableFromSession: 1},
		{ID: "mistake-rel-5", Content: "Use JSON.parse() for JSON parsing in JavaScript — eval() executes arbitrary code.", Type: domain.MemoryTypeLongTerm, Category: "best-practice", Tags: []string{"javascript", "json"}, AvailableFromSession: 1},
	}

	gotchaIDs := []string{
		"mistake-gotcha-1", "mistake-gotcha-2", "mistake-gotcha-3", "mistake-gotcha-4", "mistake-gotcha-5",
		"mistake-gotcha-6", "mistake-gotcha-7", "mistake-gotcha-8", "mistake-gotcha-9", "mistake-gotcha-10",
	}
	relevantIDs := []string{"mistake-rel-1", "mistake-rel-2", "mistake-rel-3", "mistake-rel-4", "mistake-rel-5"}

	tasks := []Task{
		{
			Description:       "Query safe git push practices",
			Query:             "git push remote branch safely without overwriting",
			RelevantMemoryIDs: relevantIDs,
			GotchaMemoryIDs:   gotchaIDs,
			TokenBudget:       2000,
		},
		{
			Description:       "Query secure secret storage",
			Query:             "store secrets credentials securely avoid committing to git",
			RelevantMemoryIDs: relevantIDs,
			GotchaMemoryIDs:   gotchaIDs,
			TokenBudget:       2000,
		},
	}

	return BenchmarkScenario{
		Name:        "mistake-prevention",
		Category:    CategoryMistakePrevention,
		ProjectID:   "benchmark",
		Description: "Measures whether wrong-approach memories are avoided in retrieval results.",
		Sessions:    5,
		Tasks:       tasks,
		Memories:    memories,
	}
}

// contextPrecision seeds 10 relevant and 10 irrelevant memories, measuring Precision@K.
func contextPrecision() BenchmarkScenario {
	memories := []SeedMemory{
		// 10 relevant memories — about Go API design
		{ID: "precision-rel-1", Content: "In Go, return errors as the last return value and check them immediately at the call site.", Type: domain.MemoryTypeLongTerm, Category: "go", Tags: []string{"go", "errors"}, AvailableFromSession: 1},
		{ID: "precision-rel-2", Content: "Use context.Context as the first parameter in Go functions that perform I/O or network calls.", Type: domain.MemoryTypeLongTerm, Category: "go", Tags: []string{"go", "context"}, AvailableFromSession: 1},
		{ID: "precision-rel-3", Content: "Define interfaces at the point of use in Go, not at the point of implementation.", Type: domain.MemoryTypeLongTerm, Category: "go", Tags: []string{"go", "interfaces"}, AvailableFromSession: 1},
		{ID: "precision-rel-4", Content: "Use table-driven tests in Go with t.Run() subtests for clear test organization.", Type: domain.MemoryTypeLongTerm, Category: "go", Tags: []string{"go", "testing"}, AvailableFromSession: 1},
		{ID: "precision-rel-5", Content: "Prefer sync.RWMutex over sync.Mutex when reads significantly outnumber writes.", Type: domain.MemoryTypeLongTerm, Category: "go", Tags: []string{"go", "concurrency"}, AvailableFromSession: 1},
		{ID: "precision-rel-6", Content: "Use 'go vet' and 'staticcheck' as part of CI to catch common Go mistakes early.", Type: domain.MemoryTypeLongTerm, Category: "go", Tags: []string{"go", "tooling"}, AvailableFromSession: 1},
		{ID: "precision-rel-7", Content: "Embed interfaces in structs to satisfy them without implementing every method.", Type: domain.MemoryTypeLongTerm, Category: "go", Tags: []string{"go", "embedding"}, AvailableFromSession: 1},
		{ID: "precision-rel-8", Content: "Use 'errors.Is' and 'errors.As' for error inspection instead of type assertions.", Type: domain.MemoryTypeLongTerm, Category: "go", Tags: []string{"go", "errors"}, AvailableFromSession: 1},
		{ID: "precision-rel-9", Content: "Avoid goroutine leaks by always providing a cancellation path via context or done channel.", Type: domain.MemoryTypeLongTerm, Category: "go", Tags: []string{"go", "goroutines"}, AvailableFromSession: 1},
		{ID: "precision-rel-10", Content: "Use 'defer' for cleanup in Go functions to ensure resources are released even on error paths.", Type: domain.MemoryTypeLongTerm, Category: "go", Tags: []string{"go", "defer"}, AvailableFromSession: 1},
		// 10 irrelevant memories — about unrelated topics
		{ID: "precision-irr-1", Content: "The Fibonacci sequence appears in sunflower seed arrangements and nautilus shells.", Type: domain.MemoryTypeEpisodic, Category: "trivia", Tags: []string{"math", "nature"}, AvailableFromSession: 1},
		{ID: "precision-irr-2", Content: "French press coffee requires a coarser grind than espresso to avoid over-extraction.", Type: domain.MemoryTypeEpisodic, Category: "trivia", Tags: []string{"coffee", "brewing"}, AvailableFromSession: 1},
		{ID: "precision-irr-3", Content: "The speed of light in a vacuum is approximately 299,792,458 metres per second.", Type: domain.MemoryTypeEpisodic, Category: "trivia", Tags: []string{"physics", "constants"}, AvailableFromSession: 1},
		{ID: "precision-irr-4", Content: "Sourdough starter requires daily feeding with equal parts flour and water by weight.", Type: domain.MemoryTypeEpisodic, Category: "trivia", Tags: []string{"baking", "fermentation"}, AvailableFromSession: 1},
		{ID: "precision-irr-5", Content: "The Great Wall of China stretches over 21,000 kilometres including all its branches.", Type: domain.MemoryTypeEpisodic, Category: "trivia", Tags: []string{"history", "geography"}, AvailableFromSession: 1},
		{ID: "precision-irr-6", Content: "Octopuses have three hearts and blue blood due to copper-based haemocyanin.", Type: domain.MemoryTypeEpisodic, Category: "trivia", Tags: []string{"biology", "marine"}, AvailableFromSession: 1},
		{ID: "precision-irr-7", Content: "The Maillard reaction between amino acids and sugars produces the browning in cooked food.", Type: domain.MemoryTypeEpisodic, Category: "trivia", Tags: []string{"chemistry", "cooking"}, AvailableFromSession: 1},
		{ID: "precision-irr-8", Content: "Mount Everest grows approximately 4mm per year due to tectonic plate movement.", Type: domain.MemoryTypeEpisodic, Category: "trivia", Tags: []string{"geology", "geography"}, AvailableFromSession: 1},
		{ID: "precision-irr-9", Content: "The human brain contains approximately 86 billion neurons connected by trillions of synapses.", Type: domain.MemoryTypeEpisodic, Category: "trivia", Tags: []string{"biology", "neuroscience"}, AvailableFromSession: 1},
		{ID: "precision-irr-10", Content: "Honey never spoils; archaeologists have found 3000-year-old honey in Egyptian tombs.", Type: domain.MemoryTypeEpisodic, Category: "trivia", Tags: []string{"food", "history"}, AvailableFromSession: 1},
	}

	relevantIDs := []string{
		"precision-rel-1", "precision-rel-2", "precision-rel-3",
		"precision-rel-4", "precision-rel-5", "precision-rel-6",
		"precision-rel-7", "precision-rel-8", "precision-rel-9", "precision-rel-10",
	}

	tasks := []Task{
		{
			Description:       "Query Go error handling patterns",
			Query:             "Go error handling return values check call site",
			RelevantMemoryIDs: relevantIDs,
			TokenBudget:       1500,
		},
		{
			Description:       "Query Go concurrency patterns",
			Query:             "Go goroutine context cancellation concurrency mutex",
			RelevantMemoryIDs: relevantIDs,
			TokenBudget:       1500,
		},
		{
			Description:       "Query Go interface and testing patterns",
			Query:             "Go interface definition testing table driven subtests",
			RelevantMemoryIDs: relevantIDs,
			TokenBudget:       1500,
		},
	}

	return BenchmarkScenario{
		Name:        "context-precision",
		Category:    CategoryContextPrecision,
		ProjectID:   "benchmark",
		Description: "Measures Precision@K with a mix of relevant Go patterns and irrelevant trivia memories.",
		Sessions:    10,
		Tasks:       tasks,
		Memories:    memories,
	}
}

// crossSessionTransfer seeds a key "pattern" memory at session 5 and measures
// whether it is retrieved in subsequent sessions.
func crossSessionTransfer() BenchmarkScenario {
	memories := []SeedMemory{
		// Base memories available from session 1
		{ID: "transfer-base-1", Content: "REST API versioning via URL path prefix (e.g. /v1/, /v2/) is the most common approach.", Type: domain.MemoryTypeLongTerm, Category: "api", Tags: []string{"api", "versioning"}, AvailableFromSession: 1},
		{ID: "transfer-base-2", Content: "Use HTTP 429 Too Many Requests with Retry-After header for rate limiting responses.", Type: domain.MemoryTypeLongTerm, Category: "api", Tags: []string{"api", "rate-limiting"}, AvailableFromSession: 1},
		{ID: "transfer-base-3", Content: "Idempotency keys in POST requests allow safe retries without duplicate side effects.", Type: domain.MemoryTypeLongTerm, Category: "api", Tags: []string{"api", "idempotency"}, AvailableFromSession: 1},
		{ID: "transfer-base-4", Content: "Use ETag and If-None-Match headers for conditional GET requests to reduce bandwidth.", Type: domain.MemoryTypeLongTerm, Category: "api", Tags: []string{"api", "caching"}, AvailableFromSession: 1},
		// The key pattern memory — available from session 5
		{ID: "transfer-pattern-1", Content: "Circuit breaker pattern: after N consecutive failures, open the circuit and return errors immediately without calling the downstream service, then probe with a single request after a timeout.", Type: domain.MemoryTypeLongTerm, Category: "api", Tags: []string{"api", "resilience", "circuit-breaker"}, AvailableFromSession: 5},
	}

	tasks := []Task{
		{
			Description:       "Query circuit breaker resilience pattern",
			Query:             "circuit breaker pattern downstream service failures resilience",
			RelevantMemoryIDs: []string{"transfer-pattern-1", "transfer-base-1"},
			TokenBudget:       2000,
		},
		{
			Description:       "Query API resilience and retry strategies",
			Query:             "API retry strategy failure handling open circuit probe",
			RelevantMemoryIDs: []string{"transfer-pattern-1", "transfer-base-3"},
			TokenBudget:       2000,
		},
	}

	return BenchmarkScenario{
		Name:        "cross-session-transfer",
		Category:    CategoryCrossSessionTransfer,
		ProjectID:   "benchmark",
		Description: "Measures whether a circuit-breaker pattern memory (available from session 5) is retrieved in later sessions.",
		Sessions:    10,
		Tasks:       tasks,
		Memories:    memories,
	}
}

// correctionSupersedes seeds a gotcha memory in session 1 and a corrective memory
// in session 5, verifying that from session 5 onward the correction ranks above the gotcha.
func correctionSupersedes() BenchmarkScenario {
	memories := []SeedMemory{
		// Gotcha memory — wrong approach, seeded from session 1
		{ID: "correction-gotcha-1", Content: "Use a global variable to share database connection across goroutines for simplicity.", Type: domain.MemoryTypeEpisodic, Category: "gotcha", Tags: []string{"go", "database", "concurrency"}, AvailableFromSession: 1},
		// Some base context memories
		{ID: "correction-base-1", Content: "database/sql in Go manages a connection pool automatically; avoid creating connections manually.", Type: domain.MemoryTypeLongTerm, Category: "go", Tags: []string{"go", "database", "sql"}, AvailableFromSession: 1},
		{ID: "correction-base-2", Content: "Use sql.DB.SetMaxOpenConns and SetMaxIdleConns to tune the connection pool size.", Type: domain.MemoryTypeLongTerm, Category: "go", Tags: []string{"go", "database", "pool"}, AvailableFromSession: 1},
		// Corrective memory — supersedes the gotcha, seeded from session 5
		{ID: "correction-fix-1", Content: "Pass *sql.DB as a dependency via constructor injection; sql.DB is safe for concurrent use and manages its own pool — no global variable needed.", Type: domain.MemoryTypeLongTerm, Category: "go", Tags: []string{"go", "database", "dependency-injection"}, AvailableFromSession: 5},
	}

	tasks := []Task{
		{
			Description:       "Query Go database connection sharing",
			Query:             "Go database connection share goroutines concurrent access",
			RelevantMemoryIDs: []string{"correction-fix-1", "correction-base-1"},
			GotchaMemoryIDs:   []string{"correction-gotcha-1"},
			CorrectionIDs:     []string{"correction-fix-1"},
			TokenBudget:       2000,
		},
		{
			Description:       "Query Go dependency injection for database",
			Query:             "Go sql.DB dependency injection constructor pool concurrent",
			RelevantMemoryIDs: []string{"correction-fix-1", "correction-base-2"},
			GotchaMemoryIDs:   []string{"correction-gotcha-1"},
			CorrectionIDs:     []string{"correction-fix-1"},
			TokenBudget:       2000,
		},
	}

	return BenchmarkScenario{
		Name:        "correction-supersedes",
		Category:    CategoryCorrectionSupersedes,
		ProjectID:   "benchmark",
		Description: "Verifies that a corrective memory (session 5) ranks above the wrong-approach gotcha memory (session 1) in retrieval results.",
		Sessions:    10,
		Tasks:       tasks,
		Memories:    memories,
	}
}
