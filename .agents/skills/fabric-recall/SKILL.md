---
name: fabric-recall
description: Find current Fabric notes, tasks, decisions, and unresolved direction without starting or synchronizing a work session. Use when the user asks what they needed to do, asks to check Fabric notes or memory, or wants to inspect live, candidate, or durable direction.
---

# Recall Fabric Notes

Answer repository-memory questions directly and keep the workflow read-only.

1. Run `fabric list --durability live --json` when the user asks about local notes or tasks.
2. Use `--candidate` or `--durable` only through the equivalent `--durability` filter when the request names those classes. Add issue, PR, area, or path filters when known.
3. Summarize actionable records first. Use `fabric list --status inactive --json` only when the user asks about history, rejected paths, or resolved work.
4. Do not run status, start a thread, preflight, sync, or read generated checkpoint files merely to recall notes.
5. Explain when no matching direction exists. Never search arbitrary repository files as a substitute for Fabric notes unless the user also asks for a general codebase search.
