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
	localEvents, err := loadLocalEvents()
	if err != nil {
		return nil, err
	}
	sharedEvents, err := loadSharedEvents()
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

func loadLocalEvents() ([]DirectionEvent, error) {
	var events []DirectionEvent
	if err := readJSONL(eventsPath, &events); err != nil {
		return nil, err
	}
	for i := range events {
		events[i].Durability = normalizeDurability(events[i].Durability)
	}
	return events, nil
}

func loadSharedEvents() ([]DirectionEvent, error) {
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
	}
	return events, nil
}

func appendEvent(event DirectionEvent) error {
	sharedPath, err := sharedEventsPath()
	if err != nil {
		return err
	}
	if sharedPath != "" {
		if err := appendLedger(sharedPath, event); err != nil {
			return err
		}
	}
	if !isDurableLike(event.Durability) {
		if sharedPath == "" {
			return appendLedger(eventsPath, event)
		}
		return nil
	}
	return appendLedger(eventsPath, event)
}

func mirrorLocalEventsToShared() error {
	sharedPath, err := sharedEventsPath()
	if err != nil || sharedPath == "" {
		return err
	}
	localEvents, err := loadLocalEvents()
	if err != nil {
		return err
	}
	if len(localEvents) == 0 {
		return nil
	}
	sharedEvents, err := loadSharedEvents()
	if err != nil {
		return err
	}
	merged := dedupeEvents(append(sharedEvents, localEvents...))
	return writeJSONL(sharedPath, merged)
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

func promoteEvent(eventID string) (DirectionEvent, error) {
	var promoted DirectionEvent
	localEvents, err := loadLocalEvents()
	if err != nil {
		return promoted, err
	}
	found := false
	for i := range localEvents {
		if localEvents[i].ID == eventID {
			if localEvents[i].Durability == DurabilityDurable {
				return promoted, fmt.Errorf("event %s is already durable", eventID)
			}
			localEvents[i].Durability = DurabilityDurable
			found = true
			promoted = localEvents[i]
		}
	}
	if !found {
		return promoted, fmt.Errorf("event %s not found or is not a promotion candidate", eventID)
	}
	if err := writeJSONL(eventsPath, localEvents); err != nil {
		return promoted, err
	}
	sharedPath, err := sharedEventsPath()
	if err != nil {
		return promoted, err
	}
	if sharedPath == "" {
		return promoted, nil
	}
	sharedEvents, err := loadSharedEvents()
	if err != nil {
		return promoted, err
	}
	for i := range sharedEvents {
		if sharedEvents[i].ID == eventID {
			sharedEvents[i].Durability = DurabilityDurable
		}
	}
	return promoted, writeJSONL(sharedPath, sharedEvents)
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
