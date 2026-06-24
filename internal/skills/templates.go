package skills

import "fmt"

type File struct {
	Path    string
	Content string
}

func Dirs() []string {
	return []string{
		"fabric-session/agents",
		"fabric-provenance/agents",
		"fabric-record-direction/agents",
		"fabric-pr-direction/agents",
		"fabric-pr-direction/references",
		"fabric-consolidate/agents",
		"fabric-publish/agents",
	}
}

func Files() []File {
	return []File{
		{Path: "fabric-session/SKILL.md", Content: fabricSessionSkill()},
		{Path: "fabric-session/agents/openai.yaml", Content: skillOpenAIYAML(
			"Fabric Session",
			"Sync direction for substantive repository work",
			"Use $fabric-session to prepare this multi-step repository implementation with shared Fabric direction.",
			false,
		)},
		{Path: "fabric-provenance/SKILL.md", Content: fabricProvenanceSkill()},
		{Path: "fabric-provenance/agents/openai.yaml", Content: skillOpenAIYAML(
			"Record Fabric Provenance",
			"Trace context exposure and causal influence",
			"Use $fabric-provenance to record why this agent action used repository direction.",
			true,
		)},
		{Path: "fabric-record-direction/SKILL.md", Content: fabricRecordDirectionSkill()},
		{Path: "fabric-record-direction/agents/openai.yaml", Content: skillOpenAIYAML(
			"Record Fabric Direction",
			"Preserve corrections and project direction",
			"Use $fabric-record-direction to preserve this correction for related agent threads.",
			true,
		)},
		{Path: "fabric-pr-direction/SKILL.md", Content: fabricPRDirectionSkill()},
		{Path: "fabric-pr-direction/agents/openai.yaml", Content: skillOpenAIYAML(
			"Mine PR Direction",
			"Extract reusable direction from GitHub PRs",
			"Use $fabric-pr-direction to review this PR and stage reusable direction for approval.",
			true,
		)},
		{Path: "fabric-pr-direction/references/github-acquisition.md", Content: githubAcquisitionReference()},
		{Path: "fabric-consolidate/SKILL.md", Content: fabricConsolidateSkill()},
		{Path: "fabric-consolidate/agents/openai.yaml", Content: skillOpenAIYAML(
			"Consolidate Fabric Direction",
			"Curate direction after work completes",
			"Use $fabric-consolidate to classify the direction left by this completed PR.",
			true,
		)},
		{Path: "fabric-publish/SKILL.md", Content: fabricPublishSkill()},
		{Path: "fabric-publish/agents/openai.yaml", Content: skillOpenAIYAML(
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
description: Prepare and synchronize Fabric threads for substantive, multi-step repository changes that may depend on shared direction. Use when beginning or resuming implementation, changing an implementation approach, continuing review-driven code work, or when the user explicitly asks to synchronize Fabric. Do not use for read-only inspection, simple questions, release or tag creation, one-off git or gh commands, or routine status checks.
---

# Fabric Session

Use Fabric when the work can create or consume repository direction. Skip this
workflow entirely for read-only inspection and one-off operational commands. A
stale or unknown current-thread pointer alone is not a reason to create a thread.

1. For substantive work, run fabric status once. If sandbox policy blocks access to .git/fabric, request scoped approval for that command and retry. Do not use another runtime store.
2. If the ongoing implementation needs shared state and there is no suitable thread, run fabric thread start with the known issue, PR, areas, and paths.
3. Run fabric preflight "<task>" with matching --issue, --pr, --area, and --path flags before implementation. Use --json only when an adapter needs the projection ID. Read .fabric/generated/TASK_DIRECTION.md.
4. Follow active direction. If the planned approach conflicts, use $fabric-record-direction to record a challenge instead of silently diverging.
5. After projected records actually enter model context, use $fabric-provenance to acknowledge exposure. Delivery alone is not exposure.
6. Run fabric sync before a meaningful implementation checkpoint, approach change, or explicit handoff. Do not sync after every command. Read .fabric/generated/SYNC_DELTA.md.
7. Use fabric continue only when resuming interrupted or review-driven PR/issue implementation. Read .fabric/generated/CONTINUATION_CONTEXT.md.

Keep user updates proportional to the task; do not narrate routine Fabric plumbing. Treat shared findings, rationale, rejected paths, and preferred paths as inputs to substantive work.
`
}

func fabricProvenanceSkill() string {
	return `---
name: fabric-provenance
description: Record and inspect Fabric context exposure and causal provenance for agent messages, actions, commits, issues, and pull requests. Use when a provider adapter exposes a projection to a model, when an agent needs to declare which direction informed or was implemented by an action, or when a user asks why an agent chose a path.
---

# Record Fabric Provenance

Keep availability and causal influence separate.

1. When a projection returned by preflight, sync, or continue actually enters model context, acknowledge it:

       fabric context acknowledge --projection "<projection-id>" --state exposed --provider "<provider>" --json

2. For an important provider object with a stable opaque ID, relate only the records actually used:

       fabric relation add --type informed_by --from "action:<provider>:<opaque-id>" --to "record:<record-id>" --actor-kind agent --actor-provider "<provider>" --actor-id "<actor-id>" --reason "<how the record influenced the action>" --json

3. Use implements instead of informed_by only when the provider object implements the direction. Use message, action, commit, issue, or pr as the source node kind as appropriate.
4. Inspect the explanation with fabric explain --node "<kind>:<provider>:<opaque-id>" --direction both --depth 4 --json.
5. Treat delivered_to and exposed_to as availability only. Never create a causal edge merely because a record was present in context.

Keep provider IDs opaque. Do not put transcripts, prompts, source code, patches, or credentials in node IDs or relation reasons.
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

       fabric note --candidate --issue "<issue>" --area "<area>" --reason "<why this matters>" "<direction>"

3. Use live only for temporary coordination. Use durable only when the human explicitly establishes long-term project direction.
4. If active direction conflicts with the proposed approach, record the dispute with fabric challenge and read .fabric/generated/CHALLENGE.md.
5. Report the created record ID and scope so later threads can trace the decision.

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
7. Run fabric continue and report created record IDs.

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

func AgentsSnippet() string {
	return RootAgentsProtocol()
}

func RootAgentsBlock() string {
	return "<!-- fabric:start -->\n" + RootAgentsProtocol() + "<!-- fabric:end -->\n"
}

func RootAgentsProtocol() string {
	return `# Direction Fabric Protocol

Fabric is the repository decision and provenance protocol for agent threads and worktrees. The CLI manages the protocol; agents are its primary clients.

- Before substantive multi-step implementation, use $fabric-session to inspect shared direction.
- Skip session setup for read-only inspection, simple questions, release or tag creation, and one-off git or gh commands.
- Before a meaningful implementation checkpoint, approach change, or handoff, run fabric sync and read .fabric/generated/SYNC_DELTA.md.
- When corrected by a human, preserve the direction and rationale with Fabric.
- Never silently violate active direction; align, ask for an exception, or record a challenge.
- Treat .fabric/ledger/events/ and the shared .git/fabric runtime as shared repository state.
- If sandbox policy blocks .git/fabric, request scoped approval for the fabric command and retry. Do not create a fallback store.

Use the repository skills for detailed workflows:

- $fabric-session
- $fabric-provenance
- $fabric-record-direction
- $fabric-pr-direction
- $fabric-consolidate
- $fabric-publish, only for explicitly requested GitHub writes

If these skills are unavailable, run fabric install-agents. It installs the managed fabric-* skills globally under ~/.agents/skills, links them into detected Cursor and Claude skill directories, and refreshes this repository protocol without changing unrelated user skills.

Commit AGENTS.md, .agents/skills/**, .fabric/config.yaml, and candidate/durable ledger direction. Do not commit generated checkpoints, thread state, or the Git-common runtime mirror.
`
}
