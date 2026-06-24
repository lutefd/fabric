# PR Review Ingest Skill

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
