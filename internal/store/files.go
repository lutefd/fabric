package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lutefd/fabric/protocol"
)

type ImmutableFileStore struct {
	WriteDir string
	ReadDirs []string
}

var _ protocol.EventStore = (*ImmutableFileStore)(nil)

func NewImmutableFileStore(writeDir string, readDirs ...string) *ImmutableFileStore {
	if len(readDirs) == 0 && writeDir != "" {
		readDirs = []string{writeDir}
	}
	return &ImmutableFileStore{WriteDir: writeDir, ReadDirs: readDirs}
}

func (s *ImmutableFileStore) Put(_ context.Context, event protocol.EventEnvelope) error {
	if s.WriteDir == "" {
		return errors.New("immutable store has no write directory")
	}
	return WriteImmutable(s.WriteDir, event)
}

func (s *ImmutableFileStore) List(_ context.Context) ([]protocol.EventEnvelope, error) {
	events, _, err := Load(s.ReadDirs...)
	return events, err
}

func WriteImmutable(dir string, event protocol.EventEnvelope) error {
	if err := event.Validate(); err != nil {
		return err
	}
	if strings.ContainsAny(event.EventID, `/\\`) {
		return errors.New("event_id contains a path separator")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	target := filepath.Join(dir, event.EventID+".json")
	if existing, err := os.ReadFile(target); err == nil {
		if bytes.Equal(existing, encoded) {
			return nil
		}
		return fmt.Errorf("immutable event %s already exists with different content", event.EventID)
	} else if !os.IsNotExist(err) {
		return err
	}

	temp, err := os.CreateTemp(dir, ".fabric-event-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(encoded); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Link(tempPath, target); err != nil {
		if os.IsExist(err) {
			existing, readErr := os.ReadFile(target)
			if readErr == nil && bytes.Equal(existing, encoded) {
				return nil
			}
		}
		return err
	}
	return nil
}

func Load(dirs ...string) ([]protocol.EventEnvelope, []string, error) {
	byID := map[string]protocol.EventEnvelope{}
	encodedByID := map[string][]byte{}
	var conflicts []string
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			raw, err := os.ReadFile(path)
			if err != nil {
				return nil, nil, err
			}
			event, err := protocol.DecodeEvent(raw)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", path, err)
			}
			canonical, _ := json.Marshal(event)
			if existing, ok := encodedByID[event.EventID]; ok {
				if !bytes.Equal(existing, canonical) {
					conflicts = append(conflicts, fmt.Sprintf("event %s has divergent immutable copies", event.EventID))
				}
				continue
			}
			byID[event.EventID] = event
			encodedByID[event.EventID] = canonical
		}
	}
	result := make([]protocol.EventEnvelope, 0, len(byID))
	for _, event := range byID {
		result = append(result, event)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].OccurredAt != result[j].OccurredAt {
			return result[i].OccurredAt < result[j].OccurredAt
		}
		return result[i].EventID < result[j].EventID
	})
	sort.Strings(conflicts)
	return result, conflicts, nil
}
