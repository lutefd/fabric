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
	var events []DirectionEvent
	if err := readJSONL(eventsPath, &events); err != nil {
		return nil, err
	}
	sharedPath, err := sharedEventsPath()
	if err != nil {
		return nil, err
	}
	if sharedPath != "" {
		if err := readJSONL(sharedPath, &events); err != nil {
			return nil, err
		}
		events = dedupeEvents(events)
	}
	sort.Slice(events, func(i, j int) bool {
		return eventNumber(events[i].ID) < eventNumber(events[j].ID)
	})
	return events, nil
}

func appendEvent(event DirectionEvent) error {
	if err := appendLedger(eventsPath, event); err != nil {
		return err
	}
	sharedPath, err := sharedEventsPath()
	if err != nil {
		return err
	}
	if sharedPath == "" {
		return nil
	}
	return appendLedger(sharedPath, event)
}

func mirrorLocalEventsToShared() error {
	sharedPath, err := sharedEventsPath()
	if err != nil || sharedPath == "" {
		return err
	}
	var localEvents []DirectionEvent
	if err := readJSONL(eventsPath, &localEvents); err != nil {
		return err
	}
	if len(localEvents) == 0 {
		return nil
	}
	var sharedEvents []DirectionEvent
	if err := readJSONL(sharedPath, &sharedEvents); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, event := range sharedEvents {
		seen[event.ID] = true
	}
	for _, event := range localEvents {
		if event.ID != "" && seen[event.ID] {
			continue
		}
		if err := appendLedger(sharedPath, event); err != nil {
			return err
		}
		seen[event.ID] = true
	}
	return nil
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
	seen := map[string]bool{}
	var deduped []DirectionEvent
	for _, event := range events {
		if event.ID != "" {
			if seen[event.ID] {
				continue
			}
			seen[event.ID] = true
		}
		deduped = append(deduped, event)
	}
	return deduped
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
