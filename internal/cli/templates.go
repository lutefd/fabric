package cli

import "fmt"

func defaultConfig(repo string) string {
	return fmt.Sprintf(`repo: %s
protocol_version: "fabric/1.0"
budgets:
  preflight_tokens: 800
  sync_delta_tokens: 300
  continuation_tokens: 700
matching:
  issue_overlap: true
  area_overlap: true
  path_overlap: true
  pr_overlap: true
areas: {}
storage:
  events: ".fabric/ledger/events"
  threads: ".git/fabric/runtime/threads"
generated:
  task_direction: ".fabric/generated/TASK_DIRECTION.md"
  sync_delta: ".fabric/generated/SYNC_DELTA.md"
  continuation_context: ".fabric/generated/CONTINUATION_CONTEXT.md"
  challenge: ".fabric/generated/CHALLENGE.md"
  consolidation: ".fabric/generated/CONSOLIDATION.md"
`, repo)
}

func generatedFiles() []string {
	return []string{
		taskPath,
		syncPath,
		continuePath,
		challengePath,
		ingestTemplatePath,
		handoffPath,
		consolidationPath,
	}
}
