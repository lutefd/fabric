# Consolidate After Merge Skill

Use this when a PR is merged, closed, abandoned, or an issue is completed.

Run:

fabric consolidate --pr "<pr>"

or:

fabric consolidate --issue "<issue>"

Read .fabric/generated/CONSOLIDATION.md.

Classify direction:

- promote when the lesson should guide future agents
- expire when the direction was temporary but valid during the task
- discard when it is too specific, noisy, wrong, or not useful
- keep candidate when it may matter later but needs more evidence

Do not promote every review comment. Durable direction should change what future agents do.
