---
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
