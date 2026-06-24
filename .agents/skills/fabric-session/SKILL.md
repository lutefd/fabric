---
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
