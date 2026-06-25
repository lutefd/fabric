package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func runClean(args []string) error {
	if len(args) == 0 {
		return errors.New(`expected "fabric clean live" or "fabric clean runtime"`)
	}
	switch args[0] {
	case "live":
		return runCleanLive(args[1:])
	case "runtime":
		return runCleanRuntime(args[1:])
	default:
		return errors.New(`expected "fabric clean live" or "fabric clean runtime"`)
	}
}

func runCleanLive(args []string) error {
	fs := flag.NewFlagSet("clean live", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	apply := fs.Bool("apply", false, "delete the previewed runtime files")
	issue := fs.String("issue", "", "select live records for an issue")
	records := stringListFlag{}
	fs.Var(&records, "record", "select a live record, repeatable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("clean live accepts flags only")
	}
	events, err := loadEvents()
	if err != nil {
		return err
	}
	explicit := map[string]bool{}
	for _, id := range records {
		explicit[id] = true
	}
	resolved := map[string]bool{}
	for _, event := range events {
		if event.Kind == "challenge_resolution" && event.Challenges != "" {
			resolved[event.Challenges] = true
		}
	}
	selected := map[string]bool{}
	for _, event := range events {
		if normalizeDurability(event.Durability) != DurabilityLive {
			continue
		}
		eligible := !isActiveEvent(event) || event.Kind == "challenge_resolution" || (event.Kind == "challenge" && resolved[event.ID])
		if explicit[event.ID] || (*issue != "" && event.Scope.Issue == *issue) || (len(explicit) == 0 && *issue == "" && eligible) {
			selected[event.ID] = true
		}
	}
	for id := range explicit {
		if !selected[id] {
			return fmt.Errorf("record %s is not a live record", id)
		}
	}
	return cleanSharedFiles("live record", selected, *apply, true)
}

func runCleanRuntime(args []string) error {
	fs := flag.NewFlagSet("clean runtime", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	apply := fs.Bool("apply", false, "delete the previewed runtime files")
	threads := stringListFlag{}
	fs.Var(&threads, "thread", "obsolete thread id, repeatable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || len(threads) == 0 {
		return errors.New("clean runtime requires at least one --thread")
	}
	selected := map[string]bool{}
	for _, id := range threads {
		selected[id] = true
	}
	return cleanSharedFiles("thread", selected, *apply, false)
}

func cleanSharedFiles(label string, selected map[string]bool, apply, includeEvents bool) error {
	if len(selected) == 0 {
		fmt.Println("Nothing eligible for cleanup.")
		setMachineResult(map[string]any{"applied": apply, "files": []string{}, "selected": []string{}})
		return nil
	}
	common, err := gitCommonDir()
	if err != nil {
		return err
	}
	roots := []string{}
	if includeEvents {
		roots = append(roots, activeEventsPath)
	}
	if common != "" {
		if includeEvents {
			roots = append(roots, filepath.Join(common, sharedEventsRel))
		}
		roots = append(roots, filepath.Join(common, sharedRuntimeRel))
	}
	var files []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				if os.IsNotExist(walkErr) {
					return nil
				}
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".json" {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			var value any
			if json.Unmarshal(data, &value) != nil {
				return nil
			}
			if containsSelected(value, selected) {
				files = append(files, path)
			}
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	sort.Strings(files)
	ids := make([]string, 0, len(selected))
	for id := range selected {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	fmt.Printf("Selected %d %s(s); %d runtime file(s).\n", len(ids), label, len(files))
	for _, id := range ids {
		fmt.Printf("- %s\n", id)
	}
	if !apply {
		fmt.Println("Preview only. Re-run with --apply to delete these ephemeral files.")
	} else {
		for _, path := range files {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		fmt.Println("Cleanup applied.")
	}
	setMachineResult(map[string]any{"applied": apply, "files": files, "selected": ids})
	return nil
}

func containsSelected(value any, selected map[string]bool) bool {
	switch typed := value.(type) {
	case string:
		return selected[typed]
	case []any:
		for _, item := range typed {
			if containsSelected(item, selected) {
				return true
			}
		}
	case map[string]any:
		for _, item := range typed {
			if containsSelected(item, selected) {
				return true
			}
		}
	}
	return false
}
