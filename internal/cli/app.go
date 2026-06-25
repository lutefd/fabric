package cli

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lutefd/fabric/internal/skills"
	"github.com/lutefd/fabric/protocol"
)

const repoAgentSkillsRoot = ".agents/skills"

func runCommand(args []string) error {
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
	case "expire":
		return runExpire(args[1:])
	case "discard":
		return runDiscard(args[1:])
	case "keep":
		return runKeep(args[1:])
	case "consolidate":
		return runConsolidate(args[1:])
	case "review":
		return runReview(args[1:])
	case "sync":
		return runSync(args[1:])
	case "status":
		return runStatus(args[1:])
	case "list":
		return runList(args[1:])
	case "clean":
		return runClean(args[1:])
	case "preflight":
		return runPreflight(args[1:])
	case "continue":
		return runContinue(args[1:])
	case "challenge":
		return runChallenge(args[1:])
	case "explain":
		return runExplain(args[1:])
	case "doctor":
		return runDoctor(args[1:])
	case "ingest-pr":
		return runIngestPR(args[1:])
	case "handoff":
		return runHandoff(args[1:])
	case "context":
		return runContext(args[1:])
	case "relation":
		return runRelation(args[1:])
	case "version":
		return runVersion(args[1:])
	case "capabilities":
		return runCapabilities(args[1:])
	case "conformance":
		return runConformance(args[1:])
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
	fabric thread list
	fabric thread use thread-b
	fabric thread clear
	fabric status
	fabric list --durability live
	fabric clean live
	fabric clean runtime --thread thread-id
	fabric note "Don't repeat this path"
	fabric note --candidate "Direction that may matter later"
	fabric note --durable "Long-term project guidance"
	fabric promote rec_... --reason "Reusable review-ingest product direction"
	fabric expire rec_... --reason "PR-local checklist item completed"
	fabric discard rec_... --reason "too specific to this PR"
	fabric keep rec_... --candidate
	fabric consolidate --pr 123
	fabric consolidate --issue FAB-3
	fabric review note --pr 123 --issue VS-123 --area virtual-store/listing "Reviewer direction"
	fabric sync --budget 300
	fabric preflight "task text" --issue VS-123 --area virtual-store/listing --budget 800
	fabric continue --pr 123 --budget 700
	fabric challenge --direction rec_... --pr 123 --issue VS-123 --proposal "New path" --reason "Why"
	fabric challenge resolve rec_... --accepted
	fabric explain --issue VS-123
	fabric explain --pr 123
	fabric doctor
	fabric ingest-pr template --pr 123 --issue VS-123 --area review-ingest
	fabric ingest-pr --pr 123 --issue VS-123 --area review-ingest --from-file review.md
	fabric ingest-pr --pr 123 --issue VS-123 --area review-ingest --stdin < review.md
	fabric handoff --pr 123 --budget 900
	fabric context acknowledge --projection prj_... --state exposed --provider codex
	fabric relation add --type informed_by --from action:codex:message-1 --to record:rec_...
	fabric version
	fabric capabilities
	fabric conformance --file event.json
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
		ledgerEventsPath,
		activeEventsPath,
		".fabric/active/issues",
		".fabric/generated",
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
		{path: agentsPath, content: skills.AgentsSnippet(), update: true},
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
	if err := installAgentSkills(repoAgentSkillsRoot, false); err != nil {
		return err
	}
	fmt.Println("Initialized Fabric in .fabric/")
	fmt.Println("Created config.yaml")
	fmt.Println("Created ledger/events/")
	fmt.Println("Created shared runtime thread store")
	fmt.Println("Created generated/")
	fmt.Println("Created .agents/skills/")
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
	if err := installAgentSkills(repoAgentSkillsRoot, true); err != nil {
		return err
	}
	globalSkillsRoot, err := globalAgentSkillsRoot()
	if err != nil {
		return err
	}
	if err := installAgentSkills(globalSkillsRoot, true); err != nil {
		return err
	}
	providerRoots, err := installDetectedProviderSkillLinks(globalSkillsRoot)
	if err != nil {
		return err
	}
	if err := installRootAgentsFile("AGENTS.md", skills.RootAgentsBlock()); err != nil {
		return err
	}
	fmt.Println("Installed Direction Fabric protocol in AGENTS.md")
	fmt.Printf("Installed Direction Fabric skills globally in %s\n", globalSkillsRoot)
	for _, root := range providerRoots {
		fmt.Printf("Linked Direction Fabric skills into %s\n", root)
	}
	return nil
}

func globalAgentSkillsRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agents", "skills"), nil
}

func installAgentSkills(root string, update bool) error {
	for _, dir := range skills.Dirs() {
		if err := mkdirAll(filepath.Join(root, dir)); err != nil {
			return err
		}
	}
	for _, file := range skills.Files() {
		path := filepath.Join(root, file.Path)
		if update {
			if err := writeFile(path, file.Content); err != nil {
				return err
			}
			continue
		}
		if err := writeFileIfMissing(path, file.Content); err != nil {
			return err
		}
	}
	return nil
}

type agentProvider struct {
	command       string
	skillsRootDir string
	markers       []string
}

func installDetectedProviderSkillLinks(sourceRoot string) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	providers := []agentProvider{
		{
			command:       "cursor",
			skillsRootDir: ".cursor",
			markers: []string{
				filepath.Join(home, ".cursor"),
				filepath.Join(home, "Applications", "Cursor.app"),
				"/Applications/Cursor.app",
			},
		},
		{
			command:       "claude",
			skillsRootDir: ".claude",
			markers: []string{
				filepath.Join(home, ".claude"),
				filepath.Join(home, ".local", "bin", "claude"),
				filepath.Join(home, "Applications", "Claude.app"),
				"/Applications/Claude.app",
			},
		},
	}

	var installedRoots []string
	for _, provider := range providers {
		if !agentProviderInstalled(provider) {
			continue
		}
		destinationRoot := filepath.Join(home, provider.skillsRootDir, "skills")
		if err := mkdirAll(destinationRoot); err != nil {
			return nil, err
		}
		for _, name := range agentSkillNames() {
			source := filepath.Join(sourceRoot, name)
			destination := filepath.Join(destinationRoot, name)
			if err := ensureSkillLink(source, destination); err != nil {
				return nil, err
			}
		}
		installedRoots = append(installedRoots, destinationRoot)
	}
	return installedRoots, nil
}

func agentProviderInstalled(provider agentProvider) bool {
	if _, err := exec.LookPath(provider.command); err == nil {
		return true
	}
	for _, marker := range provider.markers {
		if _, err := os.Stat(marker); err == nil {
			return true
		}
	}
	return false
}

func ensureSkillLink(source, destination string) error {
	info, err := os.Lstat(destination)
	if errors.Is(err, os.ErrNotExist) {
		return os.Symlink(source, destination)
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%s exists and is not a symlink; refusing to replace it", destination)
	}

	target, err := os.Readlink(destination)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(destination), target)
	}
	sourceAbsolute, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	targetAbsolute, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if filepath.Clean(sourceAbsolute) != filepath.Clean(targetAbsolute) {
		return fmt.Errorf("%s points to %s; refusing to replace it", destination, target)
	}
	return nil
}

func agentSkillNames() []string {
	return []string{
		"fabric-recall",
		"fabric-session",
		"fabric-provenance",
		"fabric-record-direction",
		"fabric-pr-direction",
		"fabric-consolidate",
		"fabric-publish",
	}
}

func runThread(args []string) error {
	if len(args) == 0 {
		return errors.New(`expected "fabric thread start", "fabric thread list", "fabric thread use", or "fabric thread clear"`)
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return errors.New("thread list accepts no arguments")
		}
		threads, err := loadThreads()
		if err != nil {
			return err
		}
		ids := make([]string, 0, len(threads))
		for id := range threads {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			fmt.Printf("%s  %s\n", id, compactThreadScope(threads[id]))
		}
		setMachineResult(map[string]any{"threads": threads, "count": len(threads)})
		return nil
	case "use":
		if len(args) != 2 {
			return errors.New("thread use requires a thread id")
		}
		threads, err := loadThreads()
		if err != nil {
			return err
		}
		if _, ok := threads[args[1]]; !ok {
			return fmt.Errorf("unknown thread %q", args[1])
		}
		if err := saveCurrentThreadID(args[1]); err != nil {
			return err
		}
		fmt.Printf("Current thread: %s.\n", args[1])
		setMachineResult(map[string]any{"current_thread": args[1]})
		return nil
	case "clear":
		if len(args) != 1 {
			return errors.New("thread clear accepts no arguments")
		}
		if err := os.Remove(currentThreadPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		fmt.Println("Current thread cleared.")
		setMachineResult(map[string]any{"current_thread": nil})
		return nil
	case "start":
	default:
		return errors.New(`expected "fabric thread start", "fabric thread list", "fabric thread use", or "fabric thread clear"`)
	}

	fs := flag.NewFlagSet("thread start", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "thread id")
	issue := fs.String("issue", "", "issue key")
	pr := fs.String("pr", "", "pull request number")
	areas := stringListFlag{}
	paths := stringListFlag{}
	fs.Var(&areas, "area", "area, repeatable")
	fs.Var(&paths, "path", "repository path or glob, repeatable")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *id == "" {
		generated, err := newThreadID()
		if err != nil {
			return err
		}
		*id = generated
	}
	if *issue == "" && *pr == "" && len(areas) == 0 && len(paths) == 0 {
		return errors.New("thread start requires --issue, --pr, --area, or --path")
	}

	record := ThreadRecord{
		ThreadID:  *id,
		CreatedAt: nowString(),
		Issue:     *issue,
		PR:        *pr,
		Areas:     areas,
		Paths:     paths,
		UpdatedAt: nowString(),
	}
	if err := saveCurrentThreadID(*id); err != nil {
		return err
	}
	if err := saveRuntimeThread(record, protocol.EventThreadStarted); err != nil {
		return err
	}
	setMachineResult(record)

	fmt.Printf("Started thread %s for issue %s, PR %s, area %s.\n", *id, emptyAsNone(*issue), emptyAsNone(*pr), areas.StringOrNone())
	fmt.Printf("Current thread: %s.\n", *id)
	return nil
}

func compactThreadScope(thread ThreadRecord) string {
	return compactScope(EventScope{Issue: thread.Issue, PR: thread.PR, Areas: thread.Areas, Paths: thread.Paths})
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
	reason := fs.String("reason", "", "rationale for the direction")
	areas := stringListFlag{}
	paths := stringListFlag{}
	fs.Var(&areas, "area", "area, repeatable")
	fs.Var(&paths, "path", "repository path or glob, repeatable")
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
	if *thread != "" && *issue == "" && *pr == "" && len(areas) == 0 && len(paths) == 0 {
		if existing, ok := threads[*thread]; ok {
			*issue = existing.Issue
			*pr = existing.PR
			areas = existing.Areas
			paths = existing.Paths
		}
	}
	if *issue == "" && *pr == "" && len(areas) == 0 && len(paths) == 0 && !*global {
		if branchIssue := issueFromBranch(); branchIssue != "" {
			*issue = branchIssue
		}
	}
	if *issue == "" && *pr == "" && len(areas) == 0 && len(paths) == 0 && !*global {
		return errors.New("no current thread found; run fabric thread start or pass --issue/--area/--path explicitly")
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
		Kind:       *kind,
		CreatedAt:  nowString(),
		Durability: durability,
		Scope: EventScope{
			Repo:   repoName(),
			Issue:  *issue,
			PR:     *pr,
			Areas:  areas,
			Paths:  paths,
			Global: *global,
		},
		Source: EventSource{
			Type:     "human",
			ThreadID: *thread,
		},
		Text:       text,
		Reason:     strings.TrimSpace(*reason),
		Confidence: "human_confirmed",
		TTL:        "until_issue_closed",
	}
	if event.Durability == DurabilityDurable && event.Reason == "" {
		event.Reason = event.Text
	}
	if err := appendEvent(&event); err != nil {
		return err
	}
	if *thread != "" {
		projection, err := createProjection("record-source", *thread,
			protocol.Scope{Repo: repoName(), Issue: event.Scope.Issue, PR: event.Scope.PR, Areas: event.Scope.Areas, Paths: event.Scope.Paths, Global: event.Scope.Global}, []DirectionEvent{event}, false)
		if err != nil {
			return err
		}
		if _, err := recordProjectionReceipt(projection, protocol.ReceiptDelivered, "fabric-cli"); err != nil {
			return err
		}
	}
	setMachineResult(event)

	stale, err := staleThreads(event, threads)
	if err != nil {
		return err
	}
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
	fmt.Println("- immutable events are written under .fabric/ledger/events/")
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
	fs := flag.NewFlagSet("promote", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	reason := fs.String("reason", "", "reason for promotion")
	if err := fs.Parse(flagsFirst(args)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("expected: fabric promote <event-id>")
	}
	if err := ensureInitialized(); err != nil {
		return err
	}
	event, err := promoteEvent(fs.Arg(0), *reason)
	if err != nil {
		return err
	}
	setMachineResult(event)
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
	matches, err := relevantUndelivered(events, thread)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		if err := writeFile(syncPath, noSyncMarkdown(resolvedThreadID)); err != nil {
			return err
		}
		projection, err := createProjection("sync", thread.ThreadID,
			protocol.Scope{Repo: repoName(), Issue: thread.Issue, PR: thread.PR, Areas: thread.Areas, Paths: thread.Paths}, nil, false)
		if err != nil {
			return err
		}
		setMachineResult(map[string]any{"thread": thread, "projection": projection, "records": []DirectionEvent{}})
		fmt.Printf("No new relevant direction for %s.\n", resolvedThreadID)
		return nil
	}
	capped, omitted := capEventsByBudget(prioritizedEvents(matches, thread.Issue, thread.PR, thread.Areas, thread.Paths), *budget)

	markdown := syncMarkdown(thread, capped, omitted)
	if err := writeFile(syncPath, markdown); err != nil {
		return err
	}
	projection, err := createProjection("sync", thread.ThreadID,
		protocol.Scope{Repo: repoName(), Issue: thread.Issue, PR: thread.PR, Areas: thread.Areas, Paths: thread.Paths}, capped, omitted)
	if err != nil {
		return err
	}
	if _, err := recordProjectionReceipt(projection, protocol.ReceiptDelivered, "fabric-cli"); err != nil {
		return err
	}
	setMachineResult(map[string]any{"thread": thread, "projection": projection, "records": capped})
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

	active := filterActionableEvents(events)
	counts := eventDurabilityCounts(active)
	result := map[string]any{"current_thread": current, "record_count": len(events), "active_record_count": len(active), "durability": counts}
	fmt.Println("Current thread:")
	if current == "" {
		fmt.Println("none")
	} else {
		fmt.Println(current)
	}
	fmt.Println()
	fmt.Println("Scope:")
	if thread, ok := threads[current]; ok {
		result["thread"] = thread
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
		matches, matchErr := relevantUndelivered(events, thread)
		if matchErr != nil {
			return matchErr
		}
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
		result["pending_records"] = matches
		fmt.Println()
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
	fmt.Println("Actionable direction:")
	fmt.Printf("- live: %d\n", counts[DurabilityLive])
	fmt.Printf("- candidate: %d\n", counts[DurabilityCandidate])
	fmt.Printf("- durable: %d\n", counts[DurabilityDurable])
	fmt.Println()
	fmt.Println("Generated files:")
	for _, path := range generatedFiles() {
		fmt.Printf("- %s\n", path)
	}
	setMachineResult(result)
	return nil
}

func runPreflight(args []string) error {
	issue, areas, paths, budget, taskParts, err := parsePreflightArgs(args)
	if err != nil {
		return err
	}
	task := strings.TrimSpace(strings.Join(taskParts, " "))
	if task == "" {
		return errors.New("preflight requires task text")
	}
	if issue == "" && len(areas) == 0 && len(paths) == 0 {
		return errors.New("preflight requires --issue, --area, or --path")
	}

	events, err := loadEvents()
	if err != nil {
		return err
	}
	matches := relevantEventsForScope(events, issue, "", areas, paths)
	capped, omitted := capEventsByBudget(prioritizedEvents(matches, issue, "", areas, paths), budget)
	markdown := preflightMarkdown(task, issue, areas, capped, omitted)
	if err := writeFile(taskPath, markdown); err != nil {
		return err
	}
	threadID, _ := loadCurrentThreadID()
	projection, err := createProjection("preflight", threadID,
		protocol.Scope{Repo: repoName(), Issue: issue, Areas: areas, Paths: paths}, capped, omitted)
	if err != nil {
		return err
	}
	if threadID != "" {
		if _, err := recordProjectionReceipt(projection, protocol.ReceiptDelivered, "fabric-cli"); err != nil {
			return err
		}
	}
	setMachineResult(map[string]any{"task": task, "projection": projection, "records": capped})
	fmt.Print(markdown)
	return nil
}

func parsePreflightArgs(args []string) (string, stringListFlag, stringListFlag, int, []string, error) {
	issue := ""
	budget := 800
	areas := stringListFlag{}
	paths := stringListFlag{}
	var taskParts []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--issue":
			i++
			if i >= len(args) {
				return "", nil, nil, 0, nil, errors.New("--issue requires a value")
			}
			issue = args[i]
		case strings.HasPrefix(arg, "--issue="):
			issue = strings.TrimPrefix(arg, "--issue=")
		case arg == "--area":
			i++
			if i >= len(args) {
				return "", nil, nil, 0, nil, errors.New("--area requires a value")
			}
			if err := areas.Set(args[i]); err != nil {
				return "", nil, nil, 0, nil, err
			}
		case strings.HasPrefix(arg, "--area="):
			if err := areas.Set(strings.TrimPrefix(arg, "--area=")); err != nil {
				return "", nil, nil, 0, nil, err
			}
		case arg == "--path":
			i++
			if i >= len(args) {
				return "", nil, nil, 0, nil, errors.New("--path requires a value")
			}
			if err := paths.Set(args[i]); err != nil {
				return "", nil, nil, 0, nil, err
			}
		case strings.HasPrefix(arg, "--path="):
			if err := paths.Set(strings.TrimPrefix(arg, "--path=")); err != nil {
				return "", nil, nil, 0, nil, err
			}
		case arg == "--budget":
			i++
			if i >= len(args) {
				return "", nil, nil, 0, nil, errors.New("--budget requires a value")
			}
			parsed, err := parseBudget(args[i])
			if err != nil {
				return "", nil, nil, 0, nil, err
			}
			budget = parsed
		case strings.HasPrefix(arg, "--budget="):
			parsed, err := parseBudget(strings.TrimPrefix(arg, "--budget="))
			if err != nil {
				return "", nil, nil, 0, nil, err
			}
			budget = parsed
		default:
			taskParts = append(taskParts, arg)
		}
	}
	return issue, areas, paths, budget, taskParts, nil
}

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	report, err := ledgerHealth()
	if err != nil {
		return err
	}
	setMachineResult(report)

	fmt.Println("Fabric doctor")
	fmt.Println()

	fmt.Println("Shared mirror:")
	if report.SharedMirrorOK {
		fmt.Println("ok")
	} else {
		fmt.Printf("error: %s\n", report.SharedMirrorError)
	}
	fmt.Println()

	fmt.Println("Durable ledger:")
	if report.DurableLedgerOK {
		fmt.Println("ok")
	} else {
		fmt.Printf("error: %s\n", report.DurableLedgerError)
	}
	fmt.Println()

	fmt.Println("Events:")
	fmt.Printf("- live: %d\n", report.Counts[DurabilityLive])
	fmt.Printf("- candidate: %d\n", report.Counts[DurabilityCandidate])
	fmt.Printf("- durable: %d\n", report.Counts[DurabilityDurable])
	fmt.Println()

	fmt.Println("Integrity:")
	printDoctorList("- invalid immutable events: ", report.InvalidLines, "none")
	printDoctorList("- durable/shared mismatch: ", report.DurableSharedMismatches, "none")
	printDoctorList("- immutable conflicts: ", report.ImmutableConflicts, "none")
	fmt.Println()

	current, err := loadCurrentThreadID()
	if err != nil {
		return err
	}
	fmt.Println("Current thread:")
	if current == "" {
		fmt.Println("none")
	} else {
		fmt.Println(current)
	}

	return nil
}

func printDoctorList(label string, items []string, none string) {
	if len(items) == 0 {
		fmt.Printf("%s%s\n", label, none)
		return
	}
	fmt.Println(label)
	for _, item := range items {
		fmt.Printf("  - %s\n", item)
	}
}

func runExplain(args []string) error {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	issue := fs.String("issue", "", "issue key")
	pr := fs.String("pr", "", "pull request number")
	nodeSpec := fs.String("node", "", "graph root as kind:id or kind:provider:id")
	direction := fs.String("direction", "both", "graph direction: incoming, outgoing, or both")
	depth := fs.Int("depth", 3, "maximum graph traversal depth")
	areas := stringListFlag{}
	paths := stringListFlag{}
	relationTypes := stringListFlag{}
	fs.Var(&areas, "area", "area, repeatable")
	fs.Var(&paths, "path", "repository path or glob, repeatable")
	fs.Var(&relationTypes, "relation", "relation type filter, repeatable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *nodeSpec != "" {
		if *direction != "incoming" && *direction != "outgoing" && *direction != "both" {
			return errors.New("--direction must be incoming, outgoing, or both")
		}
		root, err := parseNodeSpec(*nodeSpec)
		if err != nil {
			return err
		}
		graph, err := explainGraph(root, *direction, relationTypes, *depth)
		if err != nil {
			return err
		}
		setMachineResult(graph)
		printGraph(graph)
		return nil
	}
	if *issue == "" && *pr == "" && len(areas) == 0 && len(paths) == 0 {
		return errors.New("explain requires --node, --issue, --pr, --area, or --path")
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
		matched := relevantEventsForScope(events, *issue, *pr, areas, paths)
		setMachineResult(map[string]any{"records": matched})
		return printExplainPR(*pr, matched, threads)
	}
	matched := relevantEventsForScope(events, *issue, "", areas, paths)
	setMachineResult(map[string]any{"records": matched})
	return printExplain(*issue, areas, matched, threads)
}
