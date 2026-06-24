package fabric

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

func Run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "init":
		return runInit(args[1:])
	case "thread":
		return runThread(args[1:])
	case "note":
		return runNote(args[1:])
	case "sync":
		return runSync(args[1:])
	case "preflight":
		return runPreflight(args[1:])
	case "explain":
		return runExplain(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage() {
	fmt.Print(`Fabric V0

Usage:
	fabric init
	fabric thread start --id thread-b --issue VS-123 --area virtual-store/listing
	fabric note --thread thread-a --issue VS-123 --area virtual-store/listing "Don't repeat this path"
	fabric sync --thread thread-b --budget 300
	fabric preflight "task text" --issue VS-123 --area virtual-store/listing --budget 800
	fabric explain --issue VS-123
`)
}

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	dirs := []string{
		".fabric/ledger",
		".fabric/active/issues",
		".fabric/generated",
		".fabric/skills/preflight",
		".fabric/skills/sync",
		".fabric/skills/note",
	}
	for _, dir := range dirs {
		if err := mkdirAll(dir); err != nil {
			return err
		}
	}

	repo := baseName(mustGetwd())
	files := []struct {
		path    string
		content string
		touch   bool
	}{
		{path: configPath, content: defaultConfig(repo)},
		{path: eventsPath, touch: true},
		{path: threadsPath, touch: true},
		{path: ".fabric/skills/preflight/SKILL.md", content: preflightSkill()},
		{path: ".fabric/skills/sync/SKILL.md", content: syncSkill()},
		{path: ".fabric/skills/note/SKILL.md", content: noteSkill()},
		{path: agentsPath, content: agentsSnippet()},
	}
	for _, file := range files {
		var err error
		if file.touch {
			err = touchIfMissing(file.path)
		} else {
			err = writeFileIfMissing(file.path, file.content)
		}
		if err != nil {
			return err
		}
	}

	fmt.Println("Initialized Fabric in .fabric/")
	fmt.Println("Created config.yaml")
	fmt.Println("Created ledger/events.jsonl")
	fmt.Println("Created ledger/threads.jsonl")
	fmt.Println("Created generated/")
	fmt.Println("Created skills/")
	return nil
}

func runThread(args []string) error {
	if len(args) == 0 || args[0] != "start" {
		return errors.New(`expected "fabric thread start"`)
	}

	fs := flag.NewFlagSet("thread start", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "thread id")
	issue := fs.String("issue", "", "issue key")
	areas := stringListFlag{}
	fs.Var(&areas, "area", "area, repeatable")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *id == "" {
		*id = "thread_" + time.Now().Format("20060102_1504")
	}
	if *issue == "" && len(areas) == 0 {
		return errors.New("thread start requires --issue or --area")
	}

	events, err := loadEvents()
	if err != nil {
		return err
	}
	lastSeen := latestRelevantEventID(events, *issue, areas)
	record := ThreadRecord{
		ThreadID:        *id,
		CreatedAt:       nowString(),
		Issue:           *issue,
		Areas:           areas,
		LastSeenEventID: lastSeen,
	}
	if err := appendLedger(threadsPath, record); err != nil {
		return err
	}

	fmt.Printf("Started thread %s for issue %s, area %s.\n", *id, emptyAsNone(*issue), areas.StringOrNone())
	fmt.Printf("Last seen event: %s.\n", emptyAsNone(lastSeen))
	return nil
}

func runNote(args []string) error {
	fs := flag.NewFlagSet("note", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	thread := fs.String("thread", "", "source thread id")
	issue := fs.String("issue", "", "issue key")
	global := fs.Bool("global", false, "repo-wide note")
	kind := fs.String("kind", "note", "event kind")
	areas := stringListFlag{}
	fs.Var(&areas, "area", "area, repeatable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	text := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if text == "" {
		return errors.New("note text is required")
	}
	if err := ensureInitialized(); err != nil {
		return err
	}

	threads, err := loadThreads()
	if err != nil {
		return err
	}
	if *thread != "" && *issue == "" && len(areas) == 0 {
		if existing, ok := threads[*thread]; ok {
			*issue = existing.Issue
			areas = existing.Areas
		}
	}
	if *issue == "" && len(areas) == 0 && !*global {
		if branchIssue := issueFromBranch(); branchIssue != "" {
			*issue = branchIssue
		}
	}
	if *issue == "" && len(areas) == 0 && !*global {
		return errors.New("scope is required; pass --issue/--area, use a known --thread, or pass --global")
	}

	events, err := loadEvents()
	if err != nil {
		return err
	}
	event := DirectionEvent{
		ID:        nextEventID(events),
		Kind:      *kind,
		CreatedAt: nowString(),
		Scope: EventScope{
			Repo:   repoName(),
			Issue:  *issue,
			Areas:  areas,
			Global: *global,
		},
		Source: EventSource{
			Type:     "human",
			ThreadID: *thread,
		},
		Text:       text,
		Confidence: "human_confirmed",
		TTL:        "until_issue_closed",
	}
	if err := appendLedger(eventsPath, event); err != nil {
		return err
	}
	if *thread != "" {
		if sourceThread, ok := threads[*thread]; ok {
			sourceThread.LastSeenEventID = event.ID
			if err := appendLedger(threadsPath, sourceThread); err != nil {
				return err
			}
		}
	}

	stale := staleThreads(event, threads)
	fmt.Printf("Recorded direction %s.\n", event.ID)
	fmt.Printf("Marked %d related threads stale", len(stale))
	if len(stale) == 0 {
		fmt.Println(".")
		return nil
	}
	fmt.Println(":")
	for _, id := range stale {
		fmt.Printf("- %s\n", id)
	}
	return nil
}

func runSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	threadID := fs.String("thread", "", "thread id")
	budget := fs.Int("budget", 300, "token budget")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *threadID == "" {
		return errors.New("sync requires --thread")
	}

	threads, err := loadThreads()
	if err != nil {
		return err
	}
	thread, ok := threads[*threadID]
	if !ok {
		return fmt.Errorf("unknown thread %q; run fabric thread start first", *threadID)
	}
	events, err := loadEvents()
	if err != nil {
		return err
	}
	matches := relevantEventsSince(events, thread.Issue, thread.Areas, thread.LastSeenEventID)
	if len(matches) == 0 {
		if err := writeFile(syncPath, noSyncMarkdown(*threadID)); err != nil {
			return err
		}
		fmt.Printf("No new relevant direction for %s.\n", *threadID)
		return nil
	}
	capped, omitted := capEventsByBudget(matches, *budget)

	markdown := syncMarkdown(thread, capped, omitted)
	if err := writeFile(syncPath, markdown); err != nil {
		return err
	}
	thread.LastSeenEventID = matches[len(matches)-1].ID
	if err := appendLedger(threadsPath, thread); err != nil {
		return err
	}
	fmt.Print(markdown)
	return nil
}

func runPreflight(args []string) error {
	issue, areas, budget, taskParts, err := parsePreflightArgs(args)
	if err != nil {
		return err
	}
	task := strings.TrimSpace(strings.Join(taskParts, " "))
	if task == "" {
		return errors.New("preflight requires task text")
	}
	if issue == "" && len(areas) == 0 {
		return errors.New("preflight requires --issue or --area")
	}

	events, err := loadEvents()
	if err != nil {
		return err
	}
	capped, omitted := capEventsByBudget(relevantEvents(events, issue, areas), budget)
	markdown := preflightMarkdown(task, issue, areas, capped, omitted)
	if err := writeFile(taskPath, markdown); err != nil {
		return err
	}
	fmt.Print(markdown)
	return nil
}

func parsePreflightArgs(args []string) (string, stringListFlag, int, []string, error) {
	issue := ""
	budget := 800
	areas := stringListFlag{}
	var taskParts []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--issue":
			i++
			if i >= len(args) {
				return "", nil, 0, nil, errors.New("--issue requires a value")
			}
			issue = args[i]
		case strings.HasPrefix(arg, "--issue="):
			issue = strings.TrimPrefix(arg, "--issue=")
		case arg == "--area":
			i++
			if i >= len(args) {
				return "", nil, 0, nil, errors.New("--area requires a value")
			}
			if err := areas.Set(args[i]); err != nil {
				return "", nil, 0, nil, err
			}
		case strings.HasPrefix(arg, "--area="):
			if err := areas.Set(strings.TrimPrefix(arg, "--area=")); err != nil {
				return "", nil, 0, nil, err
			}
		case arg == "--budget":
			i++
			if i >= len(args) {
				return "", nil, 0, nil, errors.New("--budget requires a value")
			}
			parsed, err := parseBudget(args[i])
			if err != nil {
				return "", nil, 0, nil, err
			}
			budget = parsed
		case strings.HasPrefix(arg, "--budget="):
			parsed, err := parseBudget(strings.TrimPrefix(arg, "--budget="))
			if err != nil {
				return "", nil, 0, nil, err
			}
			budget = parsed
		default:
			taskParts = append(taskParts, arg)
		}
	}
	return issue, areas, budget, taskParts, nil
}

func runExplain(args []string) error {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	issue := fs.String("issue", "", "issue key")
	areas := stringListFlag{}
	fs.Var(&areas, "area", "area, repeatable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *issue == "" && len(areas) == 0 {
		return errors.New("explain requires --issue or --area")
	}

	events, err := loadEvents()
	if err != nil {
		return err
	}
	threads, err := loadThreads()
	if err != nil {
		return err
	}
	return printExplain(*issue, areas, relevantEvents(events, *issue, areas), threads)
}
