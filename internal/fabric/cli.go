package fabric

import (
	"bufio"
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
	case "install-agents":
		return runInstallAgents(args[1:])
	case "thread":
		return runThread(args[1:])
	case "note":
		return runNote(args[1:])
	case "promote":
		return runPromote(args[1:])
	case "review":
		return runReview(args[1:])
	case "sync":
		return runSync(args[1:])
	case "status":
		return runStatus(args[1:])
	case "preflight":
		return runPreflight(args[1:])
	case "continue":
		return runContinue(args[1:])
	case "challenge":
		return runChallenge(args[1:])
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
	fmt.Print(`Fabric

Usage:
	fabric init
	fabric install-agents
	fabric thread start --id thread-b --issue VS-123 --area virtual-store/listing
	fabric status
	fabric note "Don't repeat this path"
	fabric note --candidate "Direction that may matter later"
	fabric note --durable "Long-term project guidance"
	fabric promote evt_000018
	fabric review note --pr 123 --issue VS-123 --area virtual-store/listing "Reviewer direction"
	fabric sync --budget 300
	fabric preflight "task text" --issue VS-123 --area virtual-store/listing --budget 800
	fabric continue --pr 123 --budget 700
	fabric challenge --direction evt_000001 --pr 123 --issue VS-123 --proposal "New path" --reason "Why"
	fabric challenge resolve evt_000003 --accepted
	fabric explain --issue VS-123
	fabric explain --pr 123
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
		".fabric/skills/continue",
		".fabric/skills/challenge",
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
		update  bool
	}{
		{path: configPath, content: defaultConfig(repo)},
		{path: eventsPath, touch: true},
		{path: threadsPath, touch: true},
		{path: ".fabric/skills/preflight/SKILL.md", content: preflightSkill()},
		{path: ".fabric/skills/sync/SKILL.md", content: syncSkill()},
		{path: ".fabric/skills/note/SKILL.md", content: noteSkill()},
		{path: ".fabric/skills/continue/SKILL.md", content: continueSkill()},
		{path: ".fabric/skills/challenge/SKILL.md", content: challengeSkill()},
		{path: agentsPath, content: agentsSnippet(), update: true},
	}
	for _, file := range files {
		var err error
		if file.update {
			err = writeFile(file.path, file.content)
		} else if file.touch {
			err = touchIfMissing(file.path)
		} else {
			err = writeFileIfMissing(file.path, file.content)
		}
		if err != nil {
			return err
		}
	}
	if err := mirrorLocalEventsToShared(); err != nil {
		return err
	}

	fmt.Println("Initialized Fabric in .fabric/")
	fmt.Println("Created config.yaml")
	fmt.Println("Created ledger/events.jsonl")
	fmt.Println("Created ledger/threads.jsonl")
	fmt.Println("Created generated/")
	fmt.Println("Created skills/")
	return nil
}

func runInstallAgents(args []string) error {
	fs := flag.NewFlagSet("install-agents", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureInitialized(); err != nil {
		return err
	}
	if err := installRootAgentsFile("AGENTS.md", rootAgentsBlock()); err != nil {
		return err
	}
	fmt.Println("Installed Direction Fabric protocol in AGENTS.md")
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
	pr := fs.String("pr", "", "pull request number")
	areas := stringListFlag{}
	fs.Var(&areas, "area", "area, repeatable")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *id == "" {
		*id = "thread_" + time.Now().Format("20060102_1504")
	}
	if *issue == "" && *pr == "" && len(areas) == 0 {
		return errors.New("thread start requires --issue, --pr, or --area")
	}

	events, err := loadEvents()
	if err != nil {
		return err
	}
	lastSeen := latestRelevantEventIDForScope(events, *issue, *pr, areas)
	record := ThreadRecord{
		ThreadID:        *id,
		CreatedAt:       nowString(),
		Issue:           *issue,
		PR:              *pr,
		Areas:           areas,
		LastSeenEventID: lastSeen,
	}
	if err := appendLedger(threadsPath, record); err != nil {
		return err
	}
	if err := saveCurrentThreadID(*id); err != nil {
		return err
	}

	fmt.Printf("Started thread %s for issue %s, PR %s, area %s.\n", *id, emptyAsNone(*issue), emptyAsNone(*pr), areas.StringOrNone())
	fmt.Printf("Last seen event: %s.\n", emptyAsNone(lastSeen))
	fmt.Printf("Current thread: %s.\n", *id)
	return nil
}

func runNote(args []string) error {
	fs := flag.NewFlagSet("note", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	thread := fs.String("thread", "", "source thread id")
	issue := fs.String("issue", "", "issue key")
	pr := fs.String("pr", "", "pull request number")
	global := fs.Bool("global", false, "repo-wide note")
	kind := fs.String("kind", "note", "event kind")
	live := fs.Bool("live", false, "live only; do not persist to durable ledger")
	candidate := fs.Bool("candidate", false, "mark as promotion candidate")
	durable := fs.Bool("durable", false, "persist as durable project direction")
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

	flagCount := 0
	if *live {
		flagCount++
	}
	if *candidate {
		flagCount++
	}
	if *durable {
		flagCount++
	}
	if flagCount > 1 {
		return errors.New("only one of --live, --candidate, or --durable may be used")
	}

	threads, err := loadThreads()
	if err != nil {
		return err
	}
	if *thread == "" {
		if current, err := loadCurrentThreadID(); err != nil {
			return err
		} else if current != "" {
			*thread = current
		}
	}
	if *thread != "" && *issue == "" && *pr == "" && len(areas) == 0 {
		if existing, ok := threads[*thread]; ok {
			*issue = existing.Issue
			*pr = existing.PR
			areas = existing.Areas
		}
	}
	if *issue == "" && *pr == "" && len(areas) == 0 && !*global {
		if branchIssue := issueFromBranch(); branchIssue != "" {
			*issue = branchIssue
		}
	}
	if *issue == "" && *pr == "" && len(areas) == 0 && !*global {
		return errors.New("no current thread found; run fabric thread start --issue ... --area ... or pass --issue/--area explicitly")
	}

	events, err := loadEvents()
	if err != nil {
		return err
	}

	durability := DurabilityLive
	if *durable {
		durability = DurabilityDurable
	} else if *candidate {
		durability = DurabilityCandidate
	} else if flagCount == 0 {
		durability, err = promptDurability()
		if err != nil {
			return err
		}
	}

	event := DirectionEvent{
		ID:         nextEventID(events),
		Kind:       *kind,
		CreatedAt:  nowString(),
		Durability: durability,
		Scope: EventScope{
			Repo:   repoName(),
			Issue:  *issue,
			PR:     *pr,
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
	if err := appendEvent(event); err != nil {
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
	fmt.Printf("Recorded %s direction %s.\n", durability, event.ID)
	if isDurableLike(durability) {
		fmt.Println("Stored in durable ledger and shared across active worktrees.")
	} else {
		fmt.Println("Shared across active worktrees. Not persisted to durable ledger.")
	}
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

func promptDurability() (string, error) {
	fmt.Println()
	fmt.Println("Should this become durable project direction?")
	fmt.Println()
	fmt.Println("Durable means:")
	fmt.Println("- it is written to .fabric/ledger/events.jsonl")
	fmt.Println("- it is intended to be committed to Git")
	fmt.Println("- future agents, branches, and contributors may treat it as repo-level guidance")
	fmt.Println("- it should be stable beyond this thread/issue")
	fmt.Println()
	fmt.Println("Choose:")
	fmt.Println("[y] yes, make durable")
	fmt.Println("[n] no, live only (default)")
	fmt.Println("[l] later, leave as promotion candidate")
	fmt.Print("Choice: ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println()
		return DurabilityLive, nil
	}
	choice := strings.ToLower(strings.TrimSpace(line))
	switch choice {
	case "y", "yes", "durable":
		return DurabilityDurable, nil
	case "l", "later", "candidate":
		return DurabilityCandidate, nil
	default:
		return DurabilityLive, nil
	}
}

func runPromote(args []string) error {
	if len(args) != 1 {
		return errors.New("expected: fabric promote <event-id>")
	}
	if err := ensureInitialized(); err != nil {
		return err
	}
	event, err := promoteEvent(args[0])
	if err != nil {
		return err
	}
	fmt.Printf("Promoted %s to durable project direction.\n", event.ID)
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
	resolvedThreadID, err := resolveThreadID(*threadID)
	if err != nil {
		return err
	}

	threads, err := loadThreads()
	if err != nil {
		return err
	}
	thread, ok := threads[resolvedThreadID]
	if !ok {
		return fmt.Errorf("unknown thread %q; run fabric thread start first", resolvedThreadID)
	}
	events, err := loadEvents()
	if err != nil {
		return err
	}
	matches := relevantEventsSinceForScope(events, thread.Issue, thread.PR, thread.Areas, thread.LastSeenEventID)
	if len(matches) == 0 {
		if err := writeFile(syncPath, noSyncMarkdown(resolvedThreadID)); err != nil {
			return err
		}
		fmt.Printf("No new relevant direction for %s.\n", resolvedThreadID)
		return nil
	}
	capped, omitted := capEventsByBudget(prioritizedEvents(matches, thread.Issue, thread.PR, thread.Areas), *budget)

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

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	current, err := loadCurrentThreadID()
	if err != nil {
		return err
	}
	threads, err := loadThreads()
	if err != nil {
		return err
	}
	events, err := loadEvents()
	if err != nil {
		return err
	}

	fmt.Println("Current thread:")
	if current == "" {
		fmt.Println("none")
	} else {
		fmt.Println(current)
	}
	fmt.Println()
	fmt.Println("Scope:")
	if thread, ok := threads[current]; ok {
		fmt.Printf("issue: %s\n", emptyAsNone(thread.Issue))
		fmt.Printf("pr: %s\n", emptyAsNone(thread.PR))
		fmt.Println("areas:")
		if len(thread.Areas) == 0 {
			fmt.Println("- none")
		} else {
			for _, area := range thread.Areas {
				fmt.Printf("- %s\n", area)
			}
		}
		fmt.Println()
		matches := relevantEventsSinceForScope(events, thread.Issue, thread.PR, thread.Areas, thread.LastSeenEventID)
		fmt.Println("Sync state:")
		if len(matches) == 0 {
			fmt.Println("No new relevant directions.")
		} else if len(matches) == 1 {
			fmt.Println("1 new relevant direction available.")
			fmt.Println("Run: fabric sync")
		} else {
			fmt.Printf("%d new relevant directions available.\n", len(matches))
			fmt.Println("Run: fabric sync")
		}
	} else if current != "" {
		fmt.Printf("unknown current thread %q\n", current)
		fmt.Println()
		fmt.Println("Sync state:")
		fmt.Println("Run: fabric thread start --issue ... --area ...")
	} else {
		fmt.Println("issue: none")
		fmt.Println("pr: none")
		fmt.Println("areas:")
		fmt.Println("- none")
		fmt.Println()
		fmt.Println("Sync state:")
		fmt.Println("Run: fabric thread start --issue ... --area ...")
	}
	fmt.Println()
	fmt.Println("Generated files:")
	for _, path := range generatedFiles() {
		fmt.Printf("- %s\n", path)
	}
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
	capped, omitted := capEventsByBudget(prioritizedEvents(relevantEvents(events, issue, areas), issue, "", areas), budget)
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
	pr := fs.String("pr", "", "pull request number")
	areas := stringListFlag{}
	fs.Var(&areas, "area", "area, repeatable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *issue == "" && *pr == "" && len(areas) == 0 {
		return errors.New("explain requires --issue, --pr, or --area")
	}

	events, err := loadEvents()
	if err != nil {
		return err
	}
	threads, err := loadThreads()
	if err != nil {
		return err
	}
	if *pr != "" {
		return printExplainPR(*pr, relevantEventsForScope(events, *issue, *pr, areas), threads)
	}
	return printExplain(*issue, areas, relevantEvents(events, *issue, areas), threads)
}
