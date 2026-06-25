<!-- fabric:start -->
# Direction Fabric Protocol

Fabric is the repository decision and provenance protocol for agent threads and worktrees. The CLI manages the protocol; agents are its primary clients.

- Before substantive multi-step implementation, use $fabric-session to inspect shared direction.
- Skip session setup for read-only inspection, simple questions, release or tag creation, and one-off git or gh commands.
- Before a meaningful implementation checkpoint, approach change, or handoff, run fabric sync and read .fabric/generated/SYNC_DELTA.md.
- When corrected by a human, preserve the direction and rationale with Fabric.
- Never silently violate active direction; align, ask for an exception, or record a challenge.
- Treat .fabric/ledger/events/ and the shared .git/fabric runtime as shared repository state.
- If sandbox policy blocks .git/fabric, request scoped approval for the fabric command and retry. Do not create a fallback store.

Use the repository skills for detailed workflows:

- $fabric-recall
- $fabric-session
- $fabric-provenance
- $fabric-record-direction
- $fabric-pr-direction
- $fabric-consolidate
- $fabric-publish, only for explicitly requested GitHub writes

If these skills are unavailable, run fabric install-agents. It installs the managed fabric-* skills globally under ~/.agents/skills, links them into detected Cursor and Claude skill directories, and refreshes this repository protocol without changing unrelated user skills.

Commit AGENTS.md, .agents/skills/**, .fabric/config.yaml, and candidate/durable ledger direction. Do not commit generated checkpoints, thread state, or the Git-common runtime mirror.
<!-- fabric:end -->
