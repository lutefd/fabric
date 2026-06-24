package fabric

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var appendLedger = appendJSONL

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
	return events, nil
}

func loadEventsUnlocked() ([]DirectionEvent, error) {
	localEvents, err := loadLocalEventsUnlocked()
	if err != nil {
		return nil, err
	}
	sharedEvents, err := loadSharedEventsUnlocked()
	if err != nil {
		return nil, err
	}
	events := append(localEvents, sharedEvents...)
	events = dedupeEvents(events)
	sort.Slice(events, func(i, j int) bool {
		return eventNumber(events[i].ID) < eventNumber(events[j].ID)
	})
	return events, nil
}

func loadLocalEventsUnlocked() ([]DirectionEvent, error) {
	var events []DirectionEvent
	if err := readJSONL(eventsPath, &events); err != nil {
		return nil, err
	}
	for i := range events {
		events[i].Durability = normalizeDurability(events[i].Durability)
		events[i].Status = normalizeStatus(events[i].Status)
	}
	return events, nil
}

func loadSharedEventsUnlocked() ([]DirectionEvent, error) {
	sharedPath, err := sharedEventsPath()
	if err != nil {
		return nil, err
	}
	if sharedPath == "" {
		return nil, nil
	}
	var events []DirectionEvent
	if err := readJSONL(sharedPath, &events); err != nil {
		return nil, err
	}
	for i := range events {
		events[i].Durability = normalizeDurability(events[i].Durability)
		events[i].Status = normalizeStatus(events[i].Status)
	}
	return events, nil
}

func appendEvent(event *DirectionEvent) error {
	return withLedgerLock(func() error {
		events, err := loadEventsUnlocked()
		if err != nil {
			return err
		}
		event.ID = nextEventID(events)
		event.Durability = normalizeDurability(event.Durability)

		sharedPath, err := sharedEventsPath()
		if err != nil {
			return err
		}
		if sharedPath != "" {
			if err := appendLedger(sharedPath, *event); err != nil {
				return err
			}
		}
		if isDurableLike(event.Durability) {
			if err := appendLedger(eventsPath, *event); err != nil {
				return err
			}
		} else if sharedPath == "" {
			if err := appendLedger(eventsPath, *event); err != nil {
				return err
			}
		}
		return nil
	})
}

func mirrorLocalEventsToShared() error {
	return withLedgerLock(func() error {
		sharedPath, err := sharedEventsPath()
		if err != nil || sharedPath == "" {
			return err
		}
		localEvents, err := loadLocalEventsUnlocked()
		if err != nil {
			return err
		}
		if len(localEvents) == 0 {
			return nil
		}
		sharedEvents, err := loadSharedEventsUnlocked()
		if err != nil {
			return err
		}
		merged := dedupeEvents(append(sharedEvents, localEvents...))
		return writeJSONL(sharedPath, merged)
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
	var records []ThreadRecord
	if err := readJSONL(threadsPath, &records); err != nil {
		return nil, err
	}
	threads := map[string]ThreadRecord{}
	for _, record := range records {
		if record.ThreadID != "" {
			threads[record.ThreadID] = record
		}
	}
	return threads, nil
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

func dedupeEvents(events []DirectionEvent) []DirectionEvent {
	best := map[string]DirectionEvent{}
	for _, event := range events {
		if event.ID == "" {
			continue
		}
		existing, ok := best[event.ID]
		if !ok || durabilityRank(event.Durability) > durabilityRank(existing.Durability) {
			best[event.ID] = event
		}
	}
	var deduped []DirectionEvent
	for _, event := range best {
		deduped = append(deduped, event)
	}
	return deduped
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

func durabilityRank(d string) int {
	switch normalizeDurability(d) {
	case DurabilityDurable:
		return 3
	case DurabilityCandidate:
		return 2
	case DurabilityLive:
		return 1
	default:
		return 0
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
		event.Durability = DurabilityDurable
		return nil
	}, reason)
}

func updateEvent(eventID string, mutate func(*DirectionEvent) error, reason string) (DirectionEvent, error) {
	var updated DirectionEvent
	err := withLedgerLock(func() error {
		localEvents, err := loadLocalEventsUnlocked()
		if err != nil {
			return err
		}
		sharedPath, err := sharedEventsPath()
		if err != nil {
			return err
		}
		var sharedEvents []DirectionEvent
		if sharedPath != "" {
			sharedEvents, err = loadSharedEventsUnlocked()
			if err != nil {
				return err
			}
		}

		foundLocal := false
		for i := range localEvents {
			if localEvents[i].ID != eventID {
				continue
			}
			if err := mutate(&localEvents[i]); err != nil {
				return err
			}
			localEvents[i].ReviewedAt = nowString()
			if reason != "" {
				localEvents[i].LifecycleReason = reason
			}
			foundLocal = true
			updated = localEvents[i]
		}

		foundShared := false
		for i := range sharedEvents {
			if sharedEvents[i].ID != eventID {
				continue
			}
			if err := mutate(&sharedEvents[i]); err != nil {
				return err
			}
			sharedEvents[i].ReviewedAt = nowString()
			if reason != "" {
				sharedEvents[i].LifecycleReason = reason
			}
			foundShared = true
			if !foundLocal {
				updated = sharedEvents[i]
			}
		}

		if !foundLocal && !foundShared {
			return fmt.Errorf("event %s not found", eventID)
		}

		if foundLocal {
			if err := writeJSONL(eventsPath, localEvents); err != nil {
				return err
			}
		}
		if foundShared && sharedPath != "" {
			return writeJSONL(sharedPath, sharedEvents)
		}
		return nil
	})
	return updated, err
}

func readJSONL[T any](path string, out *[]T) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	line := 0
	for scanner.Scan() {
		line++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		var item T
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return fmt.Errorf("%s:%d: %w", path, line, err)
		}
		*out = append(*out, item)
	}
	return scanner.Err()
}

func readJSONLWithErrors[T any](path string, out *[]T) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var invalid []string
	scanner := bufio.NewScanner(file)
	line := 0
	for scanner.Scan() {
		line++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		var item T
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			invalid = append(invalid, fmt.Sprintf("%s:%d: %s", path, line, err))
			continue
		}
		*out = append(*out, item)
	}
	if err := scanner.Err(); err != nil {
		return invalid, err
	}
	return invalid, nil
}

func appendJSONL(path string, value any) error {
	if err := mkdirAll(filepath.Dir(path)); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = file.Write(append(encoded, '\n'))
	return err
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

func writeJSONL[T any](path string, values []T) error {
	if err := mkdirAll(filepath.Dir(path)); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	for _, value := range values {
		encoded, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if _, err := file.Write(append(encoded, '\n')); err != nil {
			return err
		}
	}
	return nil
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

func sharedEventsPath() (string, error) {
	common, err := gitCommonDir()
	if err != nil || common == "" {
		return "", err
	}
	return filepath.Join(common, sharedEventsRel), nil
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

func nextEventID(events []DirectionEvent) string {
	maxID := 0
	for _, event := range events {
		if n := eventNumber(event.ID); n > maxID {
			maxID = n
		}
	}
	return fmt.Sprintf("evt_%06d", maxID+1)
}

func eventNumber(id string) int {
	parts := strings.Split(id, "_")
	n, _ := strconv.Atoi(parts[len(parts)-1])
	return n
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
	SharedMirrorOK          bool
	SharedMirrorError       string
	DurableLedgerOK         bool
	DurableLedgerError      string
	Counts                  map[string]int
	DuplicateIDs            []string
	InvalidLines            []string
	DurableSharedMismatches []string
}

func ledgerHealth() (LedgerReport, error) {
	var report LedgerReport
	report.Counts = map[string]int{
		DurabilityLive:      0,
		DurabilityCandidate: 0,
		DurabilityDurable:   0,
	}

	if err := withLedgerLock(func() error {
		sharedPath, err := sharedEventsPath()
		if err != nil {
			return err
		}

		var localEvents []DirectionEvent
		localInvalid, err := readJSONLWithErrors(eventsPath, &localEvents)
		if err != nil {
			report.DurableLedgerOK = false
			report.DurableLedgerError = err.Error()
		} else if len(localInvalid) > 0 {
			report.DurableLedgerOK = false
			report.DurableLedgerError = "invalid JSONL lines detected"
		} else {
			report.DurableLedgerOK = true
		}
		report.InvalidLines = append(report.InvalidLines, localInvalid...)

		var sharedEvents []DirectionEvent
		if sharedPath == "" {
			report.SharedMirrorOK = false
			report.SharedMirrorError = "not in a git repository; no shared mirror available"
		} else {
			sharedInvalid, err := readJSONLWithErrors(sharedPath, &sharedEvents)
			if err != nil {
				report.SharedMirrorOK = false
				report.SharedMirrorError = err.Error()
			} else if len(sharedInvalid) > 0 {
				report.SharedMirrorOK = false
				report.SharedMirrorError = "invalid JSONL lines detected"
			} else {
				report.SharedMirrorOK = true
			}
			report.InvalidLines = append(report.InvalidLines, sharedInvalid...)
		}

		allEvents := dedupeEvents(append(localEvents, sharedEvents...))

		seen := map[string]int{}
		for _, event := range allEvents {
			report.Counts[normalizeDurability(event.Durability)]++
			if event.ID != "" {
				seen[event.ID]++
			}
		}
		for id, count := range seen {
			if count > 1 {
				report.DuplicateIDs = append(report.DuplicateIDs, id)
			}
		}
		sort.Strings(report.DuplicateIDs)

		sharedByID := map[string]DirectionEvent{}
		for _, event := range sharedEvents {
			if event.ID != "" {
				sharedByID[event.ID] = event
			}
		}
		for _, event := range localEvents {
			if event.ID == "" {
				continue
			}
			shared, ok := sharedByID[event.ID]
			if !ok {
				report.DurableSharedMismatches = append(report.DurableSharedMismatches, fmt.Sprintf("%s missing from shared mirror", event.ID))
				continue
			}
			if durabilityRank(event.Durability) > durabilityRank(shared.Durability) {
				report.DurableSharedMismatches = append(report.DurableSharedMismatches, fmt.Sprintf("%s durability is %s in durable ledger but %s in shared mirror", event.ID, event.Durability, shared.Durability))
			}
		}
		sort.Strings(report.DurableSharedMismatches)

		return nil
	}); err != nil {
		return report, err
	}

	return report, nil
}
