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
  consolidation: ".fabric/generated/CONSOLIDATION.md"
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

Direction has three tiers:
- live: shared across active worktrees now, not persisted to the durable ledger
- candidate: probably important, but needs review before becoming durable
- durable: long-term project guidance, committed to .fabric/ledger/events.jsonl

Interactive CLI use:
  fabric note "<direction>"

Agent use (non-interactive):
  fabric note --candidate "<direction>"

Explicit:
  fabric note --live "<direction>"
  fabric note --durable "<direction>"

Promote a candidate later:
  fabric promote <event-id>
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

func prReviewIngestSkill() string {
	return `# PR Review Ingest Skill

Use this when review feedback redirects implementation, rejects a path, prefers another path, or creates requirements that future/follow-up agents must know.

Do not ingest every review nit.

Good candidates:
- reviewer rejected an approach
- reviewer preferred a specific implementation direction
- reviewer clarified scope
- reviewer identified the correct layer/module
- reviewer explicitly said not to repeat something
- review feedback affects future/follow-up threads

Usually avoid:
- spelling fixes
- formatting nits
- one-line local cleanup
- temporary CI flakes
- comments that do not change what another agent should do next

Default:
- review direction -> candidate
- temporary review checklist item -> live

After ingesting, run:

fabric continue --pr "<pr>"

Read:

.fabric/generated/CONTINUATION_CONTEXT.md
`
}

func consolidateAfterMergeSkill() string {
	return `# Consolidate After Merge Skill

Use this when a PR is merged, closed, abandoned, or an issue is completed.

Run:

fabric consolidate --pr "<pr>"

or:

fabric consolidate --issue "<issue>"

Read .fabric/generated/CONSOLIDATION.md.

Classify direction:

- promote when the lesson should guide future agents
- expire when the direction was temporary but valid during the task
- discard when it is too specific, noisy, wrong, or not useful
- keep candidate when it may matter later but needs more evidence

Do not promote every review comment. Durable direction should change what future agents do.
`
}

func agentsSnippet() string {
	return rootAgentsProtocol()
}

func rootAgentsBlock() string {
	return "<!-- fabric:start -->\n" + rootAgentsProtocol() + "<!-- fabric:end -->\n"
}

func rootAgentsProtocol() string {
	return `# Direction Fabric Protocol

Before starting work:
- Run fabric status.
- Run fabric preflight for the task.
- Read .fabric/generated/TASK_DIRECTION.md.

Direction events are shared repo state. Do not delete .fabric/ledger/events.jsonl or treat each worktree as isolated; sibling worktrees should hear relevant notes through Fabric.

Before major checkpoints:
- Run fabric sync.
- Read .fabric/generated/SYNC_DELTA.md.

When corrected by the human:
- Run fabric note "<direction>".

Direction has three tiers:
- live: shared across active worktrees now, not persisted to the durable ledger
- candidate: probably important, but needs review before becoming durable
- durable: long-term project guidance, committed to .fabric/ledger/events.jsonl

For interactive use, run fabric note and choose the durability.
For agent use (non-interactive), prefer:
  fabric note --candidate "<direction>"
unless the human explicitly says it is temporary or durable.

Promote a candidate later:
  fabric promote <event-id>

Check ledger health:
  fabric doctor

Git hygiene rule:
  Commit project direction, not agent runtime state.

Tracked:
- AGENTS.md
- .fabric/config.yaml
- .fabric/skills/**
- .fabric/ledger/events.jsonl (candidate/durable project direction only)

Ignored:
- .fabric/active/**
- .fabric/generated/**
- .fabric/ledger/threads.jsonl
- the git-common shared mirror (.git/fabric/events.jsonl)

## PR review ingestion

When PR review redirects implementation or contains important direction, do not leave that direction only inside the PR thread.

Generate an ingest template:

fabric ingest-pr template --pr "<pr>" --issue "<issue>" --area "<area>"

Fill .fabric/generated/PR_REVIEW_INGEST.md with the review direction, then run:

fabric ingest-pr --pr "<pr>" --issue "<issue>" --area "<area>" --from-file .fabric/generated/PR_REVIEW_INGEST.md

Then run:

fabric continue --pr "<pr>"
fabric handoff --pr "<pr>"

Review ingestion should usually create candidate direction, not durable project direction.

When continuing PR/review work:
- Run fabric continue --pr "<pr>".
- Read .fabric/generated/CONTINUATION_CONTEXT.md.

When PR review redirects the implementation, record it explicitly:

fabric review note --pr "<pr>" --issue "<issue>" --area "<area>" "<review direction>"

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

## Consolidation after PR/issue completion

When a PR is merged, closed, or the issue is done, run:

fabric consolidate --pr "<pr>"

or:

fabric consolidate --issue "<issue>"

Read:

.fabric/generated/CONSOLIDATION.md

Review candidate directions and choose:

- promote: reusable project direction
- expire: useful during the task, no longer active
- discard: not useful direction / too noisy / too specific
- keep candidate: review later

Do not promote every review comment. Durable project direction should be scarce.
`
}

func generatedFiles() []string {
	return []string{
		taskPath,
		syncPath,
		continuePath,
		challengePath,
		ingestTemplatePath,
		handoffPath,
		consolidationPath,
	}
}
