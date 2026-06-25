package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/lutefd/fabric/internal/core"
	filestore "github.com/lutefd/fabric/internal/store"
)

const (
	fabricBlockStart = "<!-- fabric:start -->"
	fabricBlockEnd   = "<!-- fabric:end -->"
)

func ensureInitialized() error {
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return errors.New("not initialized; run fabric init")
		}
		return err
	}
	return nil
}

func loadEvents() ([]DirectionEvent, error) {
	if err := ensureInitialized(); err != nil {
		return nil, err
	}
	var events []DirectionEvent
	if err := withLedgerLock(func() error {
		var err error
		events, err = loadEventsUnlocked()
		return err
	}); err != nil {
		return nil, err
	}
	applyAreaPathMappings(events)
	return events, nil
}

func loadEventsUnlocked() ([]DirectionEvent, error) {
	events, _, err := loadDirectionsUnlocked()
	return events, err
}

func appendEvent(event *DirectionEvent) error {
	return withLedgerLock(func() error {
		event.Durability = normalizeDurability(event.Durability)
		return appendDirection(event)
	})
}

func ledgerLockPath() (string, error) {
	common, err := gitCommonDir()
	if err != nil {
		return "", err
	}
	if common == "" {
		return filepath.Join(filepath.Dir(currentThreadPath), "lock"), nil
	}
	return filepath.Join(common, "fabric", "lock"), nil
}

func withLedgerLock(fn func() error) error {
	lockPath, err := ledgerLockPath()
	if err != nil {
		return err
	}
	return withFileLock(lockPath, fn)
}

func withFileLock(lockPath string, fn func() error) error {
	if err := mkdirAll(filepath.Dir(lockPath)); err != nil {
		return err
	}
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	return fn()
}

func loadThreads() (map[string]ThreadRecord, error) {
	if err := ensureInitialized(); err != nil {
		return nil, err
	}
	return loadRuntimeThreads()
}

func loadCurrentThreadID() (string, error) {
	if err := ensureInitialized(); err != nil {
		return "", err
	}
	data, err := os.ReadFile(currentThreadPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func saveCurrentThreadID(threadID string) error {
	return writeFile(currentThreadPath, strings.TrimSpace(threadID)+"\n")
}

func resolveThreadID(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	current, err := loadCurrentThreadID()
	if err != nil {
		return "", err
	}
	if current == "" {
		return "", errors.New("no current thread found; run fabric thread start --issue ... --area ... or pass --thread explicitly")
	}
	return current, nil
}

func normalizeStatus(s string) string {
	if s == "" {
		return StatusActive
	}
	return s
}

func isActiveEvent(event DirectionEvent) bool {
	status := event.Status
	if status == "" {
		status = StatusActive
	}
	switch status {
	case StatusActive, "open", "accepted", "rejected":
		return true
	case StatusExpired, StatusDiscarded, StatusSuperseded:
		return false
	default:
		return false
	}
}

func filterActiveEvents(events []DirectionEvent) []DirectionEvent {
	var active []DirectionEvent
	for _, event := range events {
		if isActiveEvent(event) {
			active = append(active, event)
		}
	}
	return active
}

// actionableRecordIDs excludes historical resolution records and challenges that
// already have a resolution. They remain queryable with `fabric list --status any`.
func actionableRecordIDs(events []DirectionEvent) map[string]bool {
	resolved := map[string]bool{}
	for _, event := range events {
		if event.Kind == "challenge_resolution" && event.Challenges != "" {
			resolved[event.Challenges] = true
		}
	}
	active := map[string]bool{}
	for _, event := range events {
		if event.Kind == "challenge_resolution" || (event.Kind == "challenge" && resolved[event.ID]) {
			continue
		}
		if isActiveEvent(event) {
			active[event.ID] = true
		}
	}
	return active
}

func filterActionableEvents(events []DirectionEvent) []DirectionEvent {
	ids := actionableRecordIDs(events)
	active := make([]DirectionEvent, 0, len(ids))
	for _, event := range events {
		if ids[event.ID] {
			active = append(active, event)
		}
	}
	return active
}

func normalizeDurability(d string) string {
	if d == "" {
		return DurabilityDurable
	}
	return d
}

func isDurableLike(d string) bool {
	switch normalizeDurability(d) {
	case DurabilityDurable, DurabilityCandidate:
		return true
	default:
		return false
	}
}

func eventDurabilityCounts(events []DirectionEvent) map[string]int {
	counts := map[string]int{
		DurabilityLive:      0,
		DurabilityCandidate: 0,
		DurabilityDurable:   0,
	}
	for _, event := range events {
		counts[normalizeDurability(event.Durability)]++
	}
	return counts
}

func promoteEvent(eventID, reason string) (DirectionEvent, error) {
	return updateEvent(eventID, func(event *DirectionEvent) error {
		if event.Durability == DurabilityDurable {
			return fmt.Errorf("event %s is already durable", eventID)
		}
		if strings.TrimSpace(reason) == "" {
			return errors.New("promotion requires --reason")
		}
		_, trust := core.ActorAndTrust(*event)
		if trustRank(trust.Level) < trustRank("reviewer_confirmed") {
			return errors.New("durable promotion requires human- or reviewer-confirmed direction")
		}
		event.Durability = DurabilityDurable
		return nil
	}, reason)
}

func updateEvent(eventID string, mutate func(*DirectionEvent) error, reason string) (DirectionEvent, error) {
	var updated DirectionEvent
	err := withLedgerLock(func() error {
		events, err := loadEventsUnlocked()
		if err != nil {
			return err
		}
		var before DirectionEvent
		found := false
		for _, event := range events {
			if event.ID == eventID {
				before = event
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("event %s not found", eventID)
		}
		after := before
		if err := mutate(&after); err != nil {
			return err
		}
		after.ReviewedAt = nowString()
		if reason != "" {
			after.LifecycleReason = reason
		}
		updated, err = appendDirectionState(before, after, reason)
		return err
	})
	return updated, err
}

func writeFileIfMissing(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return writeFile(path, content)
}

func touchIfMissing(path string) error {
	if err := mkdirAll(filepath.Dir(path)); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	return file.Close()
}

func writeFile(path, content string) error {
	if err := mkdirAll(filepath.Dir(path)); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func installRootAgentsFile(path, block string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return writeFile(path, block)
		}
		return err
	}
	content := string(data)
	start := strings.Index(content, fabricBlockStart)
	end := strings.Index(content, fabricBlockEnd)
	if start == -1 || end == -1 || end < start {
		if strings.TrimSpace(content) == "" {
			return writeFile(path, block)
		}
		separator := "\n\n"
		if strings.HasSuffix(content, "\n") {
			separator = "\n"
		}
		return writeFile(path, content+separator+block)
	}
	end += len(fabricBlockEnd)
	updated := content[:start] + strings.TrimRight(block, "\n") + content[end:]
	if !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}
	return writeFile(path, updated)
}

func gitCommonDir() (string, error) {
	data, err := os.ReadFile(".git")
	if err == nil {
		gitFile := strings.TrimSpace(string(data))
		if !strings.HasPrefix(gitFile, "gitdir:") {
			return "", nil
		}
		gitDir := strings.TrimSpace(strings.TrimPrefix(gitFile, "gitdir:"))
		if !filepath.IsAbs(gitDir) {
			gitDir = filepath.Clean(filepath.Join(mustGetwd(), gitDir))
		}
		commonData, err := os.ReadFile(filepath.Join(gitDir, "commondir"))
		if err != nil {
			if os.IsNotExist(err) {
				return gitDir, nil
			}
			return "", err
		}
		common := strings.TrimSpace(string(commonData))
		if !filepath.IsAbs(common) {
			common = filepath.Clean(filepath.Join(gitDir, common))
		}
		return common, nil
	}
	info, err := os.Stat(".git")
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if !info.IsDir() {
		return "", nil
	}
	return filepath.Clean(filepath.Join(mustGetwd(), ".git")), nil
}

func mkdirAll(path string) error {
	return os.MkdirAll(path, 0o755)
}

func issueFromBranch() string {
	head, err := os.ReadFile(".git/HEAD")
	if err != nil {
		return ""
	}
	re := regexp.MustCompile(`[A-Z][A-Z0-9]+-\d+`)
	return re.FindString(string(head))
}

func repoName() string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return baseName(mustGetwd())
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "repo:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "repo:"))
		}
	}
	return baseName(mustGetwd())
}

func nowString() string {
	return time.Now().Format(time.RFC3339)
}

func mustGetwd() string {
	wd, _ := os.Getwd()
	return wd
}

func baseName(path string) string {
	return filepath.Base(path)
}

type LedgerReport struct {
	SharedMirrorOK          bool           `json:"shared_mirror_ok"`
	SharedMirrorError       string         `json:"shared_mirror_error,omitempty"`
	DurableLedgerOK         bool           `json:"durable_ledger_ok"`
	DurableLedgerError      string         `json:"durable_ledger_error,omitempty"`
	Counts                  map[string]int `json:"counts"`
	InvalidLines            []string       `json:"invalid_events,omitempty"`
	DurableSharedMismatches []string       `json:"durable_shared_mismatches,omitempty"`
	ImmutableConflicts      []string       `json:"immutable_conflicts,omitempty"`
}

func ledgerHealth() (LedgerReport, error) {
	var report LedgerReport
	report.Counts = map[string]int{
		DurabilityLive:      0,
		DurabilityCandidate: 0,
		DurabilityDurable:   0,
	}

	if err := withLedgerLock(func() error {
		events, conflicts, loadErr := loadProtocolEventsUnlocked()
		if loadErr != nil {
			report.DurableLedgerOK = false
			report.DurableLedgerError = loadErr.Error()
			report.InvalidLines = append(report.InvalidLines, loadErr.Error())
			return nil
		}
		report.DurableLedgerOK = true
		report.ImmutableConflicts = append(report.ImmutableConflicts, conflicts...)
		snapshot := core.Materialize(events)
		report.ImmutableConflicts = append(report.ImmutableConflicts, snapshot.Conflicts...)
		for _, record := range snapshot.Records {
			report.Counts[normalizeDurability(record.Record.Durability)]++
		}

		sharedDir, err := sharedEventDir()
		if err != nil {
			return err
		}
		if sharedDir == "" {
			report.SharedMirrorOK = false
			report.SharedMirrorError = "not in a git repository; no shared mirror available"
			return nil
		}

		sharedEvents, sharedConflicts, err := filestore.Load(sharedDir)
		if err != nil {
			report.SharedMirrorOK = false
			report.SharedMirrorError = err.Error()
			return nil
		}
		report.SharedMirrorOK = true
		report.ImmutableConflicts = append(report.ImmutableConflicts, sharedConflicts...)
		sharedIDs := map[string]bool{}
		for _, event := range sharedEvents {
			sharedIDs[event.EventID] = true
		}
		localEvents, _, err := filestore.Load(ledgerEventsPath)
		if err != nil {
			return err
		}
		for _, event := range localEvents {
			if !sharedIDs[event.EventID] {
				report.DurableSharedMismatches = append(report.DurableSharedMismatches, fmt.Sprintf("%s missing from shared mirror", event.EventID))
			}
		}
		sort.Strings(report.DurableSharedMismatches)
		sort.Strings(report.ImmutableConflicts)

		return nil
	}); err != nil {
		return report, err
	}

	return report, nil
}
