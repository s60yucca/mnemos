package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mnemos-dev/mnemos/internal/autopilot"
	"github.com/mnemos-dev/mnemos/internal/benchmark"
	"github.com/mnemos-dev/mnemos/internal/config"
	"github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/observe"
	"github.com/mnemos-dev/mnemos/internal/storage"
	sqlitestore "github.com/mnemos-dev/mnemos/internal/storage/sqlite"
	"github.com/spf13/cobra"
)

type CheckStatus string

const (
	CheckPass    CheckStatus = "PASS"
	CheckWarn    CheckStatus = "WARN"
	CheckFail    CheckStatus = "FAIL"
	CheckUnknown CheckStatus = "UNKNOWN"
)

type CheckReport struct {
	Status      CheckStatus   `json:"status"`
	Summary     string        `json:"summary"`
	ProjectID   string        `json:"project_id,omitempty"`
	Signals     []CheckSignal `json:"signals"`
	ActionItems []ActionItem  `json:"action_items"`
	Evidence    []Evidence    `json:"evidence"`
	Mutations   []Mutation    `json:"mutations,omitempty"`
	FixApplied  bool          `json:"fix_applied,omitempty"`
}

type CheckSignal struct {
	Name        string      `json:"name"`
	Status      CheckStatus `json:"status"`
	Value       string      `json:"value"`
	Explanation string      `json:"explanation"`
	Critical    bool        `json:"critical"`
	Scope       string      `json:"scope,omitempty"`
}

type ActionItem struct {
	Severity CheckStatus `json:"severity"`
	Message  string      `json:"message"`
	Command  string      `json:"command,omitempty"`
}

type Evidence struct {
	Source string `json:"source"`
	Detail string `json:"detail"`
}

type Mutation struct {
	MemoryID string `json:"memory_id"`
	Project  string `json:"project_id"`
	Old      string `json:"old_status"`
	New      string `json:"new_status"`
}

type checkOptions struct {
	project string
	json    bool
	verbose bool
	launch  bool
	fix     bool
}

type checkFailedError struct{}

func (checkFailedError) Error() string { return "mnemos check failed" }
func isCheckFailure(err error) bool {
	var target checkFailedError
	return errors.As(err, &target)
}

func newCheckCmd(cfg *config.Config, mnemos *core.Mnemos, buildVersion string) *cobra.Command {
	var opts checkOptions
	cmd := &cobra.Command{
		Use:           "check",
		Short:         "Verify the automatic knowledge loop",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			report := runCheck(cmd.Context(), cfg, mnemos, buildVersion, opts)
			if opts.fix && len(report.Mutations) > 0 {
				if err := applyCheckFixes(cmd.Context(), cfg.DBPath(), &report); err != nil {
					report.Signals = append(report.Signals, CheckSignal{
						Name: "safe cleanup", Status: CheckFail, Critical: true, Scope: "global",
						Value: "mutation failed", Explanation: err.Error(),
					})
					report.ActionItems = append(report.ActionItems, ActionItem{
						Severity: CheckFail, Message: "Inspect the failed cleanup mutation before retrying.",
					})
				} else {
					report.FixApplied = true
					for i := range report.Signals {
						if report.Signals[i].Name == "generated noise" {
							report.Signals[i].Status = CheckPass
							report.Signals[i].Value = "1 active report per affected project"
							report.Signals[i].Explanation = "Older generated reports were archived by the verified cleanup plan."
						}
					}
					filtered := report.ActionItems[:0]
					for _, action := range report.ActionItems {
						if action.Command != "mnemos check --fix" {
							filtered = append(filtered, action)
						}
					}
					report.ActionItems = filtered
				}
				finalizeCheckReport(&report, opts.launch)
			}
			if opts.json {
				if err := renderCheckJSON(cmd.OutOrStdout(), report); err != nil {
					return err
				}
			} else {
				renderCheckText(cmd.OutOrStdout(), report, opts.verbose)
			}
			if report.Status == CheckFail {
				return checkFailedError{}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Project ID to check")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Emit JSON only")
	cmd.Flags().BoolVar(&opts.verbose, "verbose", false, "Include detailed evidence")
	cmd.Flags().BoolVar(&opts.launch, "launch", false, "Apply public-launch readiness gates")
	cmd.Flags().BoolVar(&opts.fix, "fix", false, "Apply safe generated-report cleanup")
	return cmd
}

func runCheck(ctx context.Context, cfg *config.Config, mnemos *core.Mnemos, buildVersion string, opts checkOptions) CheckReport {
	report := CheckReport{ProjectID: opts.project}
	report.Signals = append(report.Signals, databaseSignal(cfg.DBPath()))
	memories, stats, err := loadCheckMemories(ctx, mnemos, opts.project)
	if err != nil {
		report.Signals = append(report.Signals, failedSignal("memory quality", true, projectScope(opts.project), err))
	} else {
		report.Signals = append(report.Signals, memoryQualitySignal(memories, stats))
		report.Signals = append(report.Signals, generatedNoiseSignal(memories))
		report.Mutations = planGeneratedReportCleanup(memories)
		if len(report.Mutations) > 0 && !opts.fix {
			report.ActionItems = append(report.ActionItems, ActionItem{
				Severity: CheckFail,
				Message:  fmt.Sprintf("%d older generated report(s) can be archived safely.", len(report.Mutations)),
				Command:  "mnemos check --fix",
			})
		}
		report.Signals = append(report.Signals, compileSignal(memories, cfg.Autopilot.MinAutoCompileSources, opts.project))
	}

	logPath, pathEvidence, legacyFallback := resolveFeatureLog(cfg.DataDir)
	report.Evidence = append(report.Evidence, pathEvidence...)
	if legacyFallback {
		report.Signals = append(report.Signals, CheckSignal{
			Name: "telemetry path", Status: CheckWarn, Critical: false, Scope: "global",
			Value: "legacy fallback", Explanation: "Configured feature log is missing; legacy telemetry is labeled and not merged.",
		})
	}
	events, logErr := parseLog(logPath, time.Now().UTC().Add(-7*24*time.Hour), opts.project)
	if logErr != nil {
		report.Signals = append(report.Signals, failedSignal("feature health", true, projectScope(opts.project), logErr))
		report.Signals = append(report.Signals, failedSignal("auto-inject", true, projectScope(opts.project), logErr))
	} else {
		report.Signals = append(report.Signals, featureHealthSignal(events))
		report.Signals = append(report.Signals, autoInjectSignal(events, cfg.Hook.Enabled && sessionStartHookConfigured()))
	}
	report.Signals = append(report.Signals, autopilotSignal(cfg, events))
	report.Signals = append(report.Signals, benchmarkSignal(cfg.DataDir, logPath, opts.project, opts.launch))
	report.Signals = append(report.Signals, versionSignal(buildVersion, opts.launch))

	addSignalActions(&report)
	finalizeCheckReport(&report, opts.launch)
	return report
}

func databaseSignal(dbPath string) CheckSignal {
	report := buildDoctorDatabaseReport(dbPath)
	value := "writable"
	if !report.Exists {
		value = "missing"
	} else if report.Status != CheckPass {
		value = "not writable"
	}
	explanation := "Database accepts a rolled-back write transaction."
	if len(report.Findings) > 0 {
		explanation = report.Findings[0].Message
	}
	return CheckSignal{
		Name:        "database",
		Status:      report.Status,
		Critical:    true,
		Scope:       "global",
		Value:       value,
		Explanation: explanation,
	}
}

func loadCheckMemories(ctx context.Context, mnemos *core.Mnemos, project string) ([]*domain.Memory, *storage.Stats, error) {
	stats, err := mnemos.Stats(ctx, project)
	if err != nil {
		return nil, nil, fmt.Errorf("stats: %w", err)
	}
	memories, err := mnemos.List(ctx, storage.ListQuery{
		ProjectID: project,
		Statuses:  []domain.MemoryStatus{domain.MemoryStatusActive},
		Limit:     10000,
		SortBy:    storage.SortByCreatedAt,
		SortDesc:  true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list memories: %w", err)
	}
	return memories, stats, nil
}

type evalMetrics struct {
	active, archived, lowRelevance, neverAccessed int
	quality, duplicate, stale, relevance, score   float64
}

func calculateEvalMetrics(memories []*domain.Memory, archived int) evalMetrics {
	userMemories := filterUserFacingMemories(memories)
	m := evalMetrics{active: len(userMemories), archived: archived}
	if len(userMemories) == 0 {
		return m
	}
	cutoff := time.Now().UTC().Add(-30 * 24 * time.Hour)
	for _, memory := range userMemories {
		m.quality += memory.QualityScore
		m.relevance += memory.RelevanceScore
		if memory.RelevanceScore < 0.2 {
			m.lowRelevance++
		}
		if memory.AccessCount == 0 {
			m.neverAccessed++
		}
		if memory.LastAccessedAt.Before(cutoff) {
			m.stale++
		}
	}
	m.quality /= float64(len(userMemories))
	m.relevance /= float64(len(userMemories))
	_, m.duplicate = duplicationStats(userMemories)
	staleRate := m.stale / float64(len(userMemories))
	m.score = clamp(m.quality*(1-m.duplicate*0.3)*(1-staleRate*0.4), 0, 1)
	return m
}

func memoryQualitySignal(memories []*domain.Memory, stats *storage.Stats) CheckSignal {
	metrics := calculateEvalMetrics(memories, stats.ByStatus["archived"])
	status := CheckFail
	switch {
	case metrics.active == 0:
		status = CheckFail
	case metrics.score >= 0.90:
		status = CheckPass
	case metrics.score >= 0.75:
		status = CheckWarn
	}
	return CheckSignal{
		Name:     "memory quality",
		Status:   status,
		Critical: true,
		Scope:    projectScope(stats.ProjectID),
		Value:    fmt.Sprintf("%.2f score, %d active, %.0f%% duplicates", metrics.score, metrics.active, metrics.duplicate*100),
		Explanation: fmt.Sprintf("quality %.2f, archived %d, stale %.0f%%, relevance %.2f, never accessed %d",
			metrics.quality, metrics.archived, metrics.stale*100/float64(maxInt(metrics.active, 1)), metrics.relevance, metrics.neverAccessed),
	}
}

func filterUserFacingMemories(memories []*domain.Memory) []*domain.Memory {
	result := make([]*domain.Memory, 0, len(memories))
	for _, memory := range memories {
		if isGeneratedReport(memory) {
			continue
		}
		result = append(result, memory)
	}
	return result
}

func isGeneratedReport(memory *domain.Memory) bool {
	return memory.Category == "autopilot" &&
		memory.Source == "autopilot-daemon" &&
		memory.HasTag("autopilot-report")
}

func generatedNoiseSignal(memories []*domain.Memory) CheckSignal {
	counts := map[string]int{}
	maxCount := 0
	for _, memory := range memories {
		if isGeneratedReport(memory) {
			counts[memory.ProjectID]++
			if counts[memory.ProjectID] > maxCount {
				maxCount = counts[memory.ProjectID]
			}
		}
	}
	status := CheckPass
	if maxCount > 1 {
		status = CheckFail
	}
	return CheckSignal{
		Name: "generated noise", Status: status, Critical: true, Scope: "project",
		Value:       fmt.Sprintf("max %d active report(s) per project", maxCount),
		Explanation: "Generated reports require exact category, source, and tag identity.",
	}
}

func planGeneratedReportCleanup(memories []*domain.Memory) []Mutation {
	byProject := map[string][]*domain.Memory{}
	for _, memory := range memories {
		if isGeneratedReport(memory) {
			byProject[memory.ProjectID] = append(byProject[memory.ProjectID], memory)
		}
	}
	var mutations []Mutation
	for project, reports := range byProject {
		sort.Slice(reports, func(i, j int) bool { return reports[i].CreatedAt.After(reports[j].CreatedAt) })
		for _, report := range reports[1:] {
			mutations = append(mutations, Mutation{
				MemoryID: report.ID, Project: project,
				Old: string(domain.MemoryStatusActive), New: string(domain.MemoryStatusArchived),
			})
		}
	}
	sort.Slice(mutations, func(i, j int) bool { return mutations[i].MemoryID < mutations[j].MemoryID })
	return mutations
}

func compileSignal(memories []*domain.Memory, minSources int, projectFilter string) CheckSignal {
	if minSources <= 0 {
		minSources = 5
	}
	type projectState struct {
		latest  time.Time
		sources int
	}
	projects := map[string]*projectState{}
	for _, memory := range memories {
		if memory.ProjectID == "" || (projectFilter != "" && memory.ProjectID != projectFilter) {
			continue
		}
		state := projects[memory.ProjectID]
		if state == nil {
			state = &projectState{}
			projects[memory.ProjectID] = state
		}
		if isAutoCompiled(memory) {
			if memory.CreatedAt.After(state.latest) {
				state.latest = memory.CreatedAt
			}
		}
	}
	for _, memory := range memories {
		state := projects[memory.ProjectID]
		if state == nil || isGeneratedReport(memory) || memory.Type == domain.MemoryTypeCompiled {
			continue
		}
		if state.latest.IsZero() || memory.CreatedAt.After(state.latest) {
			state.sources++
		}
	}
	if len(projects) == 0 {
		return CheckSignal{Name: "compile", Status: CheckUnknown, Critical: true, Scope: projectScope(projectFilter), Value: "no project evidence", Explanation: "No project-scoped active memories were found."}
	}
	var due, current, waiting int
	for _, state := range projects {
		switch {
		case state.latest.IsZero() && state.sources >= minSources:
			due++
		case !state.latest.IsZero() && state.sources >= minSources:
			due++
		case !state.latest.IsZero():
			current++
		default:
			waiting++
		}
	}
	status := CheckPass
	if due > 0 {
		status = CheckFail
	}
	return CheckSignal{
		Name: "compile", Status: status, Critical: true, Scope: projectScope(projectFilter),
		Value:       fmt.Sprintf("%d current, %d due, %d waiting", current, due, waiting),
		Explanation: fmt.Sprintf("Recompile after %d eligible source memories accumulate.", minSources),
	}
}

func isAutoCompiled(memory *domain.Memory) bool {
	return memory.Type == domain.MemoryTypeCompiled &&
		memory.Source == "autopilot-daemon" &&
		memory.Metadata != nil &&
		memory.Metadata["compiled_by"] == "autopilot"
}

func featureHealthSignal(events []Event) CheckSignal {
	activeDays := detectActiveDays(events)
	criticalFeatures := map[string]bool{"quality_gate": true, "dedup": true, "mmr": true}
	var firing, low, unknown, noncriticalLow int
	for name, baseline := range observe.Baselines {
		if name == "autopilot" || name == "compile" || name == "auto_inject" {
			continue
		}
		var matching []Event
		for _, event := range events {
			if event.Feature == name {
				matching = append(matching, event)
			}
		}
		classification := classifyFeatureNamed(name, matching, events, baseline, activeDays)
		switch classification {
		case StatusFiring:
			firing++
		case StatusUnknown:
			unknown++
		case StatusLow, StatusNotFiring:
			if criticalFeatures[name] {
				low++
			} else {
				noncriticalLow++
			}
		}
	}
	status := CheckPass
	if low > 0 {
		status = CheckFail
	} else if noncriticalLow > 0 {
		status = CheckWarn
	} else if unknown > 0 {
		status = CheckUnknown
	}
	return CheckSignal{
		Name: "feature health", Status: status, Critical: true, Scope: "telemetry",
		Value:       fmt.Sprintf("%d firing, %d critical low, %d noncritical low, %d unknown", firing, low, noncriticalLow, unknown),
		Explanation: "Feature baselines use operation-specific denominators; conditional features are warnings.",
	}
}

func autoInjectSignal(events []Event, hooksConfigured bool) CheckSignal {
	var canonical, attempts, payloads, timeouts, skips, failures int
	latest := ""
	for _, event := range events {
		switch event.Feature {
		case "auto_inject":
			canonical++
			latest = event.Attrs["outcome"]
			if latest == "ok" && event.Attrs["payload"] == "true" {
				payloads++
			} else if strings.HasPrefix(latest, "skipped:") {
				skips++
			} else if latest == "error" {
				failures++
			}
		case "auto_inject_attempt":
			attempts++
		case "auto_inject_timeout":
			timeouts++
		}
	}
	status := CheckFail
	explanation := "No canonical auto-inject success was observed."
	switch {
	case payloads > 0:
		status = CheckPass
		explanation = "At least one canonical event produced a payload."
	case canonical > 0 && failures == 0 && timeouts == 0:
		status = CheckWarn
		explanation = "Auto-inject ran but recent outcomes were skipped or empty."
	case canonical == 0 && attempts > 0:
		status = CheckWarn
		explanation = "Only legacy/partial auto-inject attempts were observed."
	case !hooksConfigured:
		explanation = "Session-start hooks are not configured."
	}
	return CheckSignal{
		Name: "auto-inject", Status: status, Critical: true, Scope: "telemetry",
		Value:       fmt.Sprintf("%d payloads, %d attempts, %d skips, %d timeouts", payloads, attempts, skips, timeouts),
		Explanation: explanation + optionalLatest(latest),
	}
}

func autopilotSignal(cfg *config.Config, events []Event) CheckSignal {
	if !cfg.Autopilot.Enabled {
		return CheckSignal{Name: "autopilot", Status: CheckFail, Critical: true, Scope: "global", Value: "disabled", Explanation: "Autopilot is disabled in configuration."}
	}
	state, err := autopilot.ReadStateFile(cfg.DataDir)
	if err != nil {
		return failedSignal("autopilot", true, "global", err)
	}
	latestEvent := time.Time{}
	for _, event := range events {
		if event.Feature == "autopilot" && event.Timestamp.After(latestEvent) {
			latestEvent = event.Timestamp
		}
	}
	latestRun := latestEvent
	if state != nil {
		for _, run := range state.LastRun {
			if run.After(latestRun) {
				latestRun = run
			}
		}
	}
	if latestRun.IsZero() {
		return CheckSignal{Name: "autopilot", Status: CheckFail, Critical: true, Scope: "global", Value: "not observed", Explanation: "No state file or autopilot event was found."}
	}
	tolerance := cfg.Autopilot.Interval * 2
	if tolerance < time.Hour {
		tolerance = time.Hour
	}
	age := time.Since(latestRun)
	status := CheckPass
	if age > tolerance {
		status = CheckWarn
	}
	return CheckSignal{
		Name: "autopilot", Status: status, Critical: true, Scope: "global",
		Value:       fmt.Sprintf("last run %s ago", age.Round(time.Minute)),
		Explanation: "Autopilot is enabled and measured from persisted state or canonical telemetry.",
	}
}

func benchmarkSignal(dataDir, logPath, project string, launch bool) CheckSignal {
	mode, err := benchmark.ReadBenchMode(dataDir)
	if err != nil {
		return failedSignal("benchmark", launch, "global", err)
	}
	sessions, err := extractSessions(logPath, time.Now().UTC().Add(-7*24*time.Hour), project, "", false)
	if err != nil {
		if os.IsNotExist(err) {
			return CheckSignal{Name: "benchmark", Status: CheckUnknown, Critical: launch, Scope: projectScope(project), Value: strings.ToUpper(string(mode)) + ", no sessions", Explanation: "Feature log is missing."}
		}
		return failedSignal("benchmark", launch, projectScope(project), err)
	}
	var on, off, legacy int
	for _, session := range sessions {
		switch session.Provenance {
		case "test", "fixture", "synthetic", "dry-run":
			continue
		case "production":
			if session.Mode == "on" {
				on++
			} else if session.Mode == "off" {
				off++
			}
		default:
			legacy++
		}
	}
	status := CheckPass
	switch {
	case legacy > 0 && launch:
		status = CheckUnknown
	case on >= 20 && off >= 5:
		status = CheckPass
	case on >= 10:
		status = CheckWarn
	default:
		status = CheckFail
	}
	if !launch && status == CheckFail {
		status = CheckWarn
	}
	return CheckSignal{
		Name: "benchmark", Status: status, Critical: launch, Scope: projectScope(project),
		Value:       fmt.Sprintf("%s, %d ON / %d OFF / %d legacy", strings.ToUpper(string(mode)), on, off, legacy),
		Explanation: "Only sessions with explicit production provenance count toward launch readiness.",
	}
}

func versionSignal(buildVersion string, launch bool) CheckSignal {
	executable, err := os.Executable()
	if err != nil {
		return failedSignal("version/package", launch, "global", err)
	}
	status := CheckUnknown
	explanation := "Package metadata is unavailable."
	value := buildVersion
	if buildVersion == "" || buildVersion == "dev" || strings.Contains(buildVersion, "0.0.0") {
		status = CheckWarn
		explanation = "The binary reports a development version."
	} else if packagedVersion := versionFromExecutablePath(executable); packagedVersion != "" {
		if packagedVersion == strings.TrimPrefix(buildVersion, "v") {
			status = CheckPass
			explanation = "Executable path and in-process version agree."
		} else {
			status = CheckWarn
			explanation = fmt.Sprintf("Executable path suggests version %s.", packagedVersion)
		}
	}
	return CheckSignal{
		Name: "version/package", Status: status, Critical: launch, Scope: "global",
		Value: fmt.Sprintf("%s at %s", value, executable), Explanation: explanation,
	}
}

var versionPathPattern = regexp.MustCompile(`/([0-9]+\.[0-9]+\.[0-9]+)/`)

func versionFromExecutablePath(path string) string {
	match := versionPathPattern.FindStringSubmatch(filepath.ToSlash(path))
	if len(match) == 2 {
		return match[1]
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil && resolved != path {
		match = versionPathPattern.FindStringSubmatch(filepath.ToSlash(resolved))
		if len(match) == 2 {
			return match[1]
		}
	}
	return ""
}

func resolveFeatureLog(dataDir string) (string, []Evidence, bool) {
	configured := filepath.Join(dataDir, "logs", "features.log")
	if _, err := os.Stat(configured); err == nil {
		return configured, []Evidence{{Source: "feature_log", Detail: configured}}, false
	}
	home, _ := os.UserHomeDir()
	legacy := filepath.Join(home, ".mnemos", "logs", "features.log")
	if configured != legacy {
		if _, err := os.Stat(legacy); err == nil {
			return legacy, []Evidence{
				{Source: "feature_log", Detail: "configured log missing: " + configured},
				{Source: "feature_log_legacy", Detail: legacy},
			}, true
		}
	}
	return configured, []Evidence{{Source: "feature_log", Detail: configured + " (missing)"}}, false
}

func sessionStartHookConfigured() bool {
	var candidates []string
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".claude", "settings.json"))
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, ".claude", "settings.json"))
	}
	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate)
		if err == nil && strings.Contains(string(data), "mnemos hook session-start") {
			return true
		}
	}
	return false
}

func applyCheckFixes(ctx context.Context, dbPath string, report *CheckReport) error {
	db, err := sqlitestore.OpenExistingWritable(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	store := sqlitestore.NewSQLiteStore(db)
	for i := range report.Mutations {
		mutation := &report.Mutations[i]
		memory, err := store.GetByID(ctx, mutation.MemoryID)
		if err != nil {
			return fmt.Errorf("load %s: %w", mutation.MemoryID, err)
		}
		if !isGeneratedReport(memory) || memory.ProjectID != mutation.Project || memory.Status != domain.MemoryStatusActive {
			return fmt.Errorf("cleanup plan no longer matches memory %s", mutation.MemoryID)
		}
		memory.Status = domain.MemoryStatusArchived
		memory.UpdatedAt = time.Now().UTC()
		if err := store.Update(ctx, memory); err != nil {
			return fmt.Errorf("archive %s: %w", mutation.MemoryID, err)
		}
	}
	return nil
}

func finalizeCheckReport(report *CheckReport, launch bool) {
	report.Status = aggregateCheckStatus(report.Signals, launch)
	switch report.Status {
	case CheckPass:
		report.Summary = "Autopilot knowledge loop is healthy."
	case CheckWarn:
		report.Summary = "Knowledge loop is usable, but degraded signals need attention."
	case CheckFail:
		report.Summary = "Knowledge loop has a blocking failure."
	default:
		report.Summary = "Knowledge loop readiness is unknown."
	}
	sort.SliceStable(report.ActionItems, func(i, j int) bool {
		return statusRank(report.ActionItems[i].Severity) > statusRank(report.ActionItems[j].Severity)
	})
	if len(report.ActionItems) == 0 && report.Status == CheckPass {
		report.ActionItems = []ActionItem{{Severity: CheckPass, Message: "No immediate action required."}}
	}
}

func aggregateCheckStatus(signals []CheckSignal, launch bool) CheckStatus {
	result := CheckPass
	for _, signal := range signals {
		status := signal.Status
		if status == CheckUnknown {
			if signal.Critical && launch {
				status = CheckFail
			} else {
				status = CheckWarn
			}
		}
		if statusRank(status) > statusRank(result) {
			result = status
		}
	}
	return result
}

func addSignalActions(report *CheckReport) {
	existing := map[string]bool{}
	existingCommands := map[string]bool{}
	for _, action := range report.ActionItems {
		existing[action.Message] = true
		if action.Command != "" {
			existingCommands[action.Command] = true
		}
	}
	for _, signal := range report.Signals {
		if signal.Status != CheckWarn && signal.Status != CheckFail && signal.Status != CheckUnknown {
			continue
		}
		var message, command string
		switch signal.Name {
		case "memory quality":
			message = "Review low-quality, stale, or duplicate memories with `mnemos eval`."
			command = "mnemos eval"
		case "database":
			message = "Run doctor database to inspect DB path, permissions, and writable probe."
			command = "mnemos doctor database"
		case "feature health":
			message = "Inspect feature denominators and recent activity with `mnemos health`."
			command = "mnemos health"
		case "auto-inject":
			message = "Run or verify the session-start hook and inspect auto-inject outcomes."
		case "autopilot":
			message = "Run doctor to inspect autopilot leadership, process state, and database writability."
			command = "mnemos doctor all"
		case "compile":
			message = "Run or inspect autopilot compilation for projects with pending sources."
			command = "mnemos autopilot run"
		case "benchmark":
			message = "Collect provenance-marked ON and OFF benchmark sessions before launch."
			command = "mnemos bench status"
		case "version/package":
			message = "Verify build version and installed package path."
			command = "mnemos version"
		case "generated noise":
			message = "Run with --fix to apply safe cleanup."
			command = "mnemos check --fix"
		case "telemetry path":
			message = "Run doctor logs to inspect feature log path and writability."
			command = "mnemos doctor logs"
		}
		if message != "" && !existing[message] && (command == "" || !existingCommands[command]) {
			report.ActionItems = append(report.ActionItems, ActionItem{Severity: signal.Status, Message: message, Command: command})
			existing[message] = true
			if command != "" {
				existingCommands[command] = true
			}
		}
	}
}

func renderCheckJSON(w io.Writer, report CheckReport) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func renderCheckText(w io.Writer, report CheckReport, verbose bool) {
	fmt.Fprintf(w, "Mnemos Check - %s\n%s\n", report.Status, report.Summary)
	if report.ProjectID != "" {
		fmt.Fprintf(w, "Project: %s\n", report.ProjectID)
	}
	fmt.Fprintln(w, "\nSignals")
	for _, signal := range report.Signals {
		fmt.Fprintf(w, "[%s] %s: %s\n", signal.Status, signal.Name, signal.Value)
		if verbose {
			fmt.Fprintf(w, "  %s (%s)\n", signal.Explanation, signal.Scope)
		}
	}
	fmt.Fprintln(w, "\nAction Items")
	for i, action := range report.ActionItems {
		fmt.Fprintf(w, "%d. %s", i+1, action.Message)
		if action.Command != "" {
			fmt.Fprintf(w, " [%s]", action.Command)
		}
		fmt.Fprintln(w)
	}
	if len(report.Mutations) > 0 {
		if report.FixApplied || verbose {
			fmt.Fprintln(w, "\nMutations")
			for _, mutation := range report.Mutations {
				fmt.Fprintf(w, "- %s project=%s %s -> %s\n", mutation.MemoryID, mutation.Project, mutation.Old, mutation.New)
			}
		} else {
			fmt.Fprintf(w, "\nPlanned cleanup: %d generated report(s)\n", len(report.Mutations))
		}
	}
	if verbose && len(report.Evidence) > 0 {
		fmt.Fprintln(w, "\nEvidence")
		for _, evidence := range report.Evidence {
			fmt.Fprintf(w, "- %s: %s\n", evidence.Source, evidence.Detail)
		}
	}
}

func failedSignal(name string, critical bool, scope string, err error) CheckSignal {
	return CheckSignal{Name: name, Status: CheckFail, Critical: critical, Scope: scope, Value: "read failed", Explanation: err.Error()}
}

func statusRank(status CheckStatus) int {
	switch status {
	case CheckFail:
		return 4
	case CheckWarn:
		return 3
	case CheckUnknown:
		return 2
	default:
		return 1
	}
}

func projectScope(project string) string {
	if project == "" {
		return "all-projects"
	}
	return "project:" + project
}

func optionalLatest(latest string) string {
	if latest == "" {
		return ""
	}
	return " Latest outcome: " + latest + "."
}

func clamp(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
