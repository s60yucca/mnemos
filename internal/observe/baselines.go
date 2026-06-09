package observe

// ActiveDayThreshold is the minimum MCP calls per day to count as an active day
const ActiveDayThreshold = 10

// Baseline defines expected firing rates for a feature
type Baseline struct {
	MinDaily        int     // minimum events per active day (for always-on features)
	RatioVsMCPCalls float64 // minimum ratio vs MCP calls (for per-call features, 0 = not applicable)
	Expected        string  // human-readable description
}

// Baselines maps feature names to their expected behavior baselines
var Baselines = map[string]Baseline{
	"quality_gate": {
		MinDaily:        0,
		RatioVsMCPCalls: 0.95,
		Expected:        "Every store operation (95%+ of stores)",
	},
	"dedup": {
		MinDaily:        0,
		RatioVsMCPCalls: 0.95,
		Expected:        "Every store operation (95%+ of stores)",
	},
	"summarize": {
		MinDaily:        2,
		RatioVsMCPCalls: 0.0,
		Expected:        "Auto-summarization on store",
	},
	"file_link": {
		MinDaily:        0,
		RatioVsMCPCalls: 0.30,
		Expected:        "Memories with file paths (30%+ of stores)",
	},
	"mmr": {
		MinDaily:        0,
		RatioVsMCPCalls: 0.95,
		Expected:        "Every context assembly (95%+ of context calls)",
	},
	"autopilot": {
		MinDaily:        24,
		RatioVsMCPCalls: 0.0,
		Expected:        "Daemon runs every 15min during active hours",
	},
	"compile": {
		MinDaily:        1,
		RatioVsMCPCalls: 0.0,
		Expected:        "Autopilot compile checks and compile operations",
	},
	"decay": {
		MinDaily:        1,
		RatioVsMCPCalls: 0.0,
		Expected:        "Daily lifecycle maintenance",
	},
	"topic_shift": {
		MinDaily:        2,
		RatioVsMCPCalls: 0.0,
		Expected:        "Session topic changes",
	},
	"auto_inject": {
		MinDaily:        3,
		RatioVsMCPCalls: 0.1,
		Expected:        "At least 3 auto-inject payloads per active day; ratio ~0.1 vs total MCP calls",
	},
}
