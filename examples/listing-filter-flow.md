# Listing Filter Flow

This is the smallest cross-thread correction loop using an issue-shaped task.

```bash
fabric init
fabric thread start --id thread-a --issue VS-123 --area virtual-store/listing
fabric thread start --id thread-b --issue VS-123 --area virtual-store/listing
```

Thread A starts down the wrong path. The human records the correction once:

```bash
fabric note --thread thread-a --issue VS-123 --area virtual-store/listing "Don't create a second listing endpoint; extend the existing one or escalate API direction"
```

Fabric marks Thread B stale:

```text
Recorded candidate direction rec_....
Marked 1 related threads stale:
- thread-b
```

Thread B syncs before continuing:

```bash
fabric sync --thread thread-b --budget 300
```

Thread B receives only the relevant delta:

```md
# Sync Delta

Thread:
thread-b

New relevant direction since last sync:

1. Don't create a second listing endpoint; extend the existing one or escalate API direction

Source:
Human note from related thread thread-a.

Why it applies:
- Same issue: VS-123
- Same area: virtual-store/listing

Next action:
Adjust your plan before continuing.
```

Success is not a complete memory system. Success is that this tiny packet changes Thread B's next move.
