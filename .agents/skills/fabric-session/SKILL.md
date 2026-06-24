---
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
