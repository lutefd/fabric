---
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
