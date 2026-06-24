package fabric

import "fmt"

func defaultConfig(repo string) string {
	return fmt.Sprintf(`repo: %s
budgets:
  preflight_tokens: 800
  sync_delta_tokens: 300
  continuation_tokens: 700
matching:
  issue_overlap: true
  area_overlap: true
  pr_overlap: true
storage:
  events: ".fabric/ledger/events.jsonl"
  threads: ".fabric/ledger/threads.jsonl"
generated:
  task_direction: ".fabric/generated/TASK_DIRECTION.md"
  sync_delta: ".fabric/generated/SYNC_DELTA.md"
  continuation_context: ".fabric/generated/CONTINUATION_CONTEXT.md"
  challenge: ".fabric/generated/CHALLENGE.md"
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

func continueSkill() string {
	return `# Direction Fabric Continue

When continuing PR, review, issue, or area work in a fresh thread, run fabric continue, then read .fabric/generated/CONTINUATION_CONTEXT.md.
`
}

func challengeSkill() string {
	return `# Direction Fabric Challenge

If planned work conflicts with active direction, record the explicit dispute with fabric challenge, then read .fabric/generated/CHALLENGE.md.
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

When PR review redirects the implementation, record it explicitly:

fabric review note --pr "<pr>" --issue "<issue>" --area "<area>" "<review direction>"

When continuing PR/review work in a fresh or follow-up thread, run:

fabric continue --pr "<pr>" --thread "<thread-id>" --budget 700

Read:

.fabric/generated/CONTINUATION_CONTEXT.md

## Challenging direction

If your planned approach conflicts with active direction, do not silently proceed.

Choose one:

1. Align with existing direction
2. Ask for a scoped exception
3. Record a challenge

To record a challenge, run:

fabric challenge \
  --direction "<event-id>" \
  --issue "<issue>" \
  --pr "<pr>" \
  --area "<area>" \
  --proposal "<new proposed path>" \
  --reason "<why the existing direction may not apply>"

Read:

.fabric/generated/CHALLENGE.md

Mention the challenge in the PR or handoff.
`
}
