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

type agentSkillFile struct {
	path    string
	content string
}

func agentSkillDirs() []string {
	return []string{
		"fabric-session/agents",
		"fabric-record-direction/agents",
		"fabric-pr-direction/agents",
		"fabric-pr-direction/references",
		"fabric-consolidate/agents",
		"fabric-publish/agents",
	}
}

func agentSkillFiles() []agentSkillFile {
	return []agentSkillFile{
		{path: "fabric-session/SKILL.md", content: fabricSessionSkill()},
		{path: "fabric-session/agents/openai.yaml", content: skillOpenAIYAML(
			"Fabric Session",
			"Start and synchronize Fabric agent sessions",
			"Use $fabric-session to prepare this repository task with the current Fabric direction.",
			true,
		)},
		{path: "fabric-record-direction/SKILL.md", content: fabricRecordDirectionSkill()},
		{path: "fabric-record-direction/agents/openai.yaml", content: skillOpenAIYAML(
			"Record Fabric Direction",
			"Preserve corrections and project direction",
			"Use $fabric-record-direction to preserve this correction for related agent threads.",
			true,
		)},
		{path: "fabric-pr-direction/SKILL.md", content: fabricPRDirectionSkill()},
		{path: "fabric-pr-direction/agents/openai.yaml", content: skillOpenAIYAML(
			"Mine PR Direction",
			"Extract reusable direction from GitHub PRs",
			"Use $fabric-pr-direction to review this PR and stage reusable direction for approval.",
			true,
		)},
		{path: "fabric-pr-direction/references/github-acquisition.md", content: githubAcquisitionReference()},
		{path: "fabric-consolidate/SKILL.md", content: fabricConsolidateSkill()},
		{path: "fabric-consolidate/agents/openai.yaml", content: skillOpenAIYAML(
			"Consolidate Fabric Direction",
			"Curate direction after work completes",
			"Use $fabric-consolidate to classify the direction left by this completed PR.",
			true,
		)},
		{path: "fabric-publish/SKILL.md", content: fabricPublishSkill()},
		{path: "fabric-publish/agents/openai.yaml", content: skillOpenAIYAML(
			"Publish Fabric Context",
			"Publish approved Fabric context to GitHub",
			"Use $fabric-publish to preview and publish the approved Fabric handoff to GitHub.",
			false,
		)},
	}
}

func skillOpenAIYAML(displayName, shortDescription, defaultPrompt string, allowImplicit bool) string {
	return fmt.Sprintf(`interface:
  display_name: %q
  short_description: %q
  default_prompt: %q
policy:
  allow_implicit_invocation: %t
`, displayName, shortDescription, defaultPrompt, allowImplicit)
}

func fabricSessionSkill() string {
	return `---
name: fabric-session
description: Initialize, resume, and synchronize Fabric agent threads. Use before starting repository work, resuming a PR or issue, changing approach, opening a PR, or checking for new project direction shared by other threads and worktrees.
---

# Fabric Session

Use Fabric as the repository decision protocol before acting.

1. Run fabric status. If sandbox policy blocks access to .git/fabric, request scoped approval for the fabric command and retry. Do not use another runtime store.
2. If there is no suitable current thread, run fabric thread start with the known PR, issue, and areas.
3. Run fabric preflight with the task and the same scope. Read .fabric/generated/TASK_DIRECTION.md.
4. Follow active direction. If the planned approach conflicts, use $fabric-record-direction to record a challenge instead of silently diverging.
5. Before changing approach, opening a PR, or resuming later, run fabric sync and read .fabric/generated/SYNC_DELTA.md.
6. When continuing PR or issue work, run fabric continue and read .fabric/generated/CONTINUATION_CONTEXT.md.

Treat shared findings, rationale, rejected paths, and preferred paths as inputs to the current thread, not as optional background.
`
}

func fabricRecordDirectionSkill() string {
	return `---
name: fabric-record-direction
description: Record human corrections, reusable repository direction, or explicit challenges in Fabric. Use when a user redirects work, rejects an approach, establishes a constraint, explains why a choice was made, or asks to preserve a finding for other agent threads and worktrees.
---

# Record Fabric Direction

Record only guidance that changes what another agent should do or explains why a path was chosen.

1. Preserve the smallest complete statement, including rationale and scope.
2. Default uncertain reusable guidance to candidate:

       fabric note --candidate --issue "<issue>" --area "<area>" "<direction and rationale>"

3. Use live only for temporary coordination. Use durable only when the human explicitly establishes long-term project direction.
4. If active direction conflicts with the proposed approach, record the dispute with fabric challenge and read .fabric/generated/CHALLENGE.md.
5. Report the created event ID and scope so later threads can trace the decision.

Do not record transcripts, code dumps, formatting nits, or facts that do not affect future action.
`
}

func fabricPRDirectionSkill() string {
	return `---
name: fabric-pr-direction
description: Mine GitHub pull request discussion or a bounded list of historical PRs for reusable repository direction, rejected paths, preferred implementations, and rationale, then ingest only user-approved items into Fabric. Use for PR review ingestion, review-driven handoffs, curated decision-history seeding, or tracing why an implementation choice was made.
---

# Mine PR Direction

Keep GitHub acquisition outside Fabric. Fabric validates and stores approved direction.

## Acquire context

1. Accept one PR or an explicit bounded PR list. Never discover or crawl an entire repository implicitly.
2. Prefer an available native GitHub connector or provider tool and keep acquisition read-only.
3. If no connector is available, follow references/github-acquisition.md and use authenticated gh directly.
4. Stop with setup guidance if neither path is available.
5. Collect the PR body, linked issues, discussion, submitted reviews, inline comments, changed files, commits, and checks.

## Extract direction

Keep feedback that changes future agent behavior: rejected approaches, preferred layers or designs, clarified scope, durable constraints, or findings another thread would otherwise rediscover.

Skip nits, formatting, local cleanup, transient CI failures, raw transcripts, and comments that do not change future action. Preserve the reviewer, source URL, rationale, rejected paths, and preferred paths as evidence.

## Stage and approve

1. Process each PR independently.
2. Generate the existing ingest template:

       fabric ingest-pr template --pr "<pr>" --issue "<issue>" --area "<area>"

3. Fill .fabric/generated/PR_REVIEW_INGEST.md with the proposed items and evidence.
4. Validate without mutating the ledger:

       fabric ingest-pr --pr "<pr>" --issue "<issue>" --area "<area>" --from-file .fabric/generated/PR_REVIEW_INGEST.md --dry-run

5. Present a numbered proposal list in chat. Do not ingest before the user selects items.
6. Remove unapproved items, repeat the dry run, then run the same command without --dry-run.
7. Run fabric continue and report created event IDs.

For bounded historical seeding, repeat per PR so work can resume after partial failure. Finish with accepted, skipped, and failed PR counts. Mining never comments on or edits GitHub.
`
}

func githubAcquisitionReference() string {
	return `# GitHub Context Acquisition

## Native connector

Prefer a GitHub connector or provider-native GitHub tool already available to the agent. Use read operations only. Fetch PR metadata, body, linked issues, conversation comments, reviews, inline review comments, files, commits, checks, authors, and stable URLs.

Do not require a particular connector name. Inspect the available tools and use the narrowest read capability that provides the required context.

## gh fallback

Verify installation and authentication:

    command -v gh
    gh auth status
    gh repo view --json nameWithOwner

Fetch the main PR context:

    gh pr view "<pr>" --json number,title,body,url,state,author,reviewDecision,comments,reviews,files,commits,statusCheckRollup,closingIssuesReferences

Fetch paginated inline review comments when the main response does not include them:

    gh api --paginate "repos/{owner}/{repo}/pulls/<pr>/comments"

Use gh output as agent context; do not add a Fabric GitHub wrapper or store credentials. If gh is missing or unauthenticated, stop and tell the user which prerequisite failed.
`
}

func fabricConsolidateSkill() string {
	return `---
name: fabric-consolidate
description: Review and classify Fabric direction after a PR, issue, or agent task is merged, closed, abandoned, or completed. Use to promote reusable decisions, expire temporary coordination, discard noise, retain uncertain candidates, and preserve a sparse decision history.
---

# Consolidate Fabric Direction

1. Run fabric consolidate with the completed PR or issue.
2. Read .fabric/generated/CONSOLIDATION.md.
3. Present proposed lifecycle actions before mutating direction.
4. After approval, promote reusable repository decisions, expire completed temporary direction, discard noise, and keep uncertain candidates.
5. Preserve evidence and rationale needed to explain later why an agent followed or avoided a path.

Durable direction must stay scarce. Do not promote every review comment or task-local requirement.
`
}

func fabricPublishSkill() string {
	return `---
name: fabric-publish
description: Publish an existing Fabric handoff, continuation summary, or managed context block to GitHub. Use only when the user explicitly asks to comment on a pull request or update its body with Fabric context; never invoke for read-only mining or ordinary handoff generation.
---

# Publish Fabric Context

This workflow creates an external side effect and requires explicit approval at action time.

1. Identify the destination repository and PR.
2. Generate or read the requested Fabric artifact, such as .fabric/generated/HANDOFF.md or .fabric/generated/CONSOLIDATION.md.
3. Show the exact content and whether it will create a comment or replace a managed PR-body block.
4. Obtain explicit user approval immediately before publishing.
5. Prefer an available native GitHub connector. Otherwise verify gh auth status and use gh pr comment --body-file or gh pr edit --body-file.
6. For body updates, preserve all content outside the <!-- fabric:start --> and <!-- fabric:end --> markers.
7. Report the resulting GitHub URL when available.

Never publish automatically during mining, ingestion, continuation, handoff, or consolidation.
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

Fabric is the repository decision and provenance protocol for agent threads and worktrees. The CLI manages the protocol; agents are its primary clients.

- Before work, run fabric status and fabric preflight, then read .fabric/generated/TASK_DIRECTION.md.
- Before changing approach, opening a PR, or resuming work, run fabric sync and read .fabric/generated/SYNC_DELTA.md.
- When corrected by a human, preserve the direction and rationale with Fabric.
- Never silently violate active direction; align, ask for an exception, or record a challenge.
- Treat .fabric/ledger/events.jsonl and the shared .git/fabric runtime as shared repository state.
- If sandbox policy blocks .git/fabric, request scoped approval for the fabric command and retry. Do not create a fallback store.

Use the repository skills for detailed workflows:

- $fabric-session
- $fabric-record-direction
- $fabric-pr-direction
- $fabric-consolidate
- $fabric-publish, only for explicitly requested GitHub writes

If these skills are unavailable, run fabric install-agents. It installs the managed fabric-* skills globally under ~/.agents/skills and refreshes this repository protocol without changing unrelated user skills.

Commit AGENTS.md, .agents/skills/**, .fabric/config.yaml, and candidate/durable ledger direction. Do not commit generated checkpoints, thread state, or the Git-common runtime mirror.
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
