package fabric

import "fmt"

func defaultConfig(repo string) string {
	return fmt.Sprintf(`repo: %s
budgets:
  preflight_tokens: 800
  sync_delta_tokens: 300
matching:
  issue_overlap: true
  area_overlap: true
storage:
  events: ".fabric/ledger/events.jsonl"
  threads: ".fabric/ledger/threads.jsonl"
generated:
  task_direction: ".fabric/generated/TASK_DIRECTION.md"
  sync_delta: ".fabric/generated/SYNC_DELTA.md"
`, repo)
}

func preflightSkill() string {
	return `# Direction Fabric Preflight

Before starting work, run fabric preflight with the task, issue, and area, then read .fabric/generated/TASK_DIRECTION.md.
`
}

func syncSkill() string {
	return `# Direction Fabric Sync

Before implementation, before changing approach, before opening a PR, and before resuming old work, run fabric sync, then read .fabric/generated/SYNC_DELTA.md.
`
}

func noteSkill() string {
	return `# Direction Fabric Note

When the human gives project direction, record it with fabric note using the current thread, issue, and area.
`
}

func agentsSnippet() string {
	return `# Direction Fabric Protocol

Before starting work, run:

fabric preflight "<task>" --issue "<issue>" --area "<area>" --budget 800

Read:

.fabric/generated/TASK_DIRECTION.md

Before implementation, before changing approach, before opening a PR, and before resuming old work, run:

fabric sync --thread "<thread-id>" --budget 300

Read:

.fabric/generated/SYNC_DELTA.md

When the human gives project direction, record it:

fabric note --thread "<thread-id>" --issue "<issue>" --area "<area>" "<direction>"

Do not silently ignore active direction. If your planned approach conflicts with direction, stop and ask whether to align, request an exception, or challenge the direction.
`
}
