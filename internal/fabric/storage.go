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
	sort.Slice(events, func(i, j int) bool {
		return eventNumber(events[i].ID) < eventNumber(events[j].ID)
	})
	return events, nil
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
