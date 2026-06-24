---
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
