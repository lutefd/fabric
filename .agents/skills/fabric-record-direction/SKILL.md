---
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
