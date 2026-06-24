package cli

import (
	"bufio"
	"errors"
	"strings"
)

const ingestTemplatePath = ".fabric/generated/PR_REVIEW_INGEST.md"

type IngestFile struct {
	PR       string
	Issue    string
	Areas    []string
	Items    []IngestItem
	Warnings []string
}

type IngestItem struct {
	Type           string
	Durability     string
	ReviewSays     string
	RejectedPaths  []string
	PreferredPaths []string
	Reason         string
	Evidence       []EvidenceRef
}

func parseIngestFile(content string) (IngestFile, error) {
	var ingest IngestFile
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0
	var currentSection string
	var currentItem *IngestItem
	var currentField string
	var fieldBuffer []string

	flushField := func() {
		if currentItem == nil || currentField == "" {
			return
		}
		text := strings.TrimSpace(strings.Join(fieldBuffer, "\n"))
		switch currentField {
		case "type":
			currentItem.Type = strings.ToLower(strings.TrimSpace(text))
		case "durability":
			currentItem.Durability = strings.ToLower(strings.TrimSpace(text))
		case "review says":
			currentItem.ReviewSays = text
		case "rejected paths":
			currentItem.RejectedPaths = parseBulletList(text)
		case "preferred paths":
			currentItem.PreferredPaths = parseBulletList(text)
		case "reason":
			currentItem.Reason = text
		case "evidence":
			currentItem.Evidence = parseEvidence(text)
		}
		currentField = ""
		fieldBuffer = nil
	}

	flushItem := func() {
		if currentItem == nil {
			return
		}
		flushField()
		if strings.TrimSpace(currentItem.ReviewSays) != "" {
			ingest.Items = append(ingest.Items, *currentItem)
		} else {
			ingest.Warnings = append(ingest.Warnings, "skipped direction item with no 'Review says'")
		}
		currentItem = nil
	}

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "# ") && strings.Contains(trimmed, "PR Review Ingest") {
			continue
		}

		if strings.HasPrefix(trimmed, "PR:") {
			ingest.PR = strings.TrimSpace(strings.TrimPrefix(trimmed, "PR:"))
			continue
		}
		if strings.HasPrefix(trimmed, "Issue:") {
			ingest.Issue = strings.TrimSpace(strings.TrimPrefix(trimmed, "Issue:"))
			continue
		}
		if trimmed == "Areas:" {
			currentSection = "areas"
			continue
		}
		if currentSection == "areas" {
			if strings.HasPrefix(trimmed, "-") {
				area := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
				if area != "" {
					ingest.Areas = append(ingest.Areas, area)
				}
				continue
			}
			if trimmed != "" {
				currentSection = ""
			}
		}

		if trimmed == "## Review directions" {
			flushItem()
			currentSection = "directions"
			continue
		}

		if currentSection == "directions" && strings.HasPrefix(trimmed, "### ") {
			flushItem()
			currentItem = &IngestItem{}
			continue
		}

		if currentItem != nil {
			if isFieldLabel(trimmed) {
				flushField()
				label := extractFieldLabel(trimmed)
				currentField = strings.ToLower(label)
				for _, known := range []string{"Type", "Durability", "Review says", "Rejected paths", "Preferred paths", "Reason", "Evidence"} {
					if strings.EqualFold(label, known) {
						parts := strings.SplitN(trimmed, ":", 2)
						if len(parts) == 2 {
							value := strings.TrimSpace(parts[1])
							if value != "" {
								fieldBuffer = append(fieldBuffer, value)
							}
						}
						break
					}
				}
				continue
			}
			fieldBuffer = append(fieldBuffer, line)
		}
	}
	flushItem()

	if err := scanner.Err(); err != nil {
		return ingest, err
	}
	if len(ingest.Items) == 0 {
		return ingest, errors.New("no valid review direction items found")
	}
	return ingest, nil
}

func extractFieldLabel(line string) string {
	line = strings.TrimPrefix(line, "**")
	line = strings.TrimSuffix(line, "**")
	parts := strings.SplitN(line, ":", 2)
	label := strings.TrimSpace(parts[0])
	return label
}

func isFieldLabel(line string) bool {
	labels := []string{
		"Type:",
		"Durability:",
		"Review says:",
		"Rejected paths:",
		"Preferred paths:",
		"Reason:",
		"Evidence:",
	}
	for _, label := range labels {
		if strings.HasPrefix(line, label) || strings.HasPrefix(line, "**"+label+"**") {
			return true
		}
	}
	return false
}

func parseBulletList(text string) []string {
	var items []string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*") {
			item := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(trimmed, "-"), "*"))
			if item != "" {
				items = append(items, item)
			}
		}
	}
	return items
}

func parseEvidence(text string) []EvidenceRef {
	var refs []EvidenceRef
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "-") {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
		}
		ref := EvidenceRef{}
		if strings.HasPrefix(trimmed, "reviewer comment:") {
			ref.Type = "reviewer_comment"
			ref.Text = strings.TrimSpace(strings.TrimPrefix(trimmed, "reviewer comment:"))
		} else if strings.HasPrefix(trimmed, "url:") {
			ref.Type = "url"
			ref.URL = strings.TrimSpace(strings.TrimPrefix(trimmed, "url:"))
		} else {
			ref.Type = "note"
			ref.Text = trimmed
		}
		if ref.Text != "" || ref.URL != "" {
			refs = append(refs, ref)
		}
	}
	return refs
}

func inferIngestDurability(item IngestItem) string {
	if item.Durability != "" {
		switch item.Durability {
		case DurabilityLive, DurabilityCandidate, DurabilityDurable:
			return item.Durability
		}
	}
	switch item.Type {
	case "rejection", "preference", "review_direction":
		return DurabilityCandidate
	case "requirement", "checklist", "test request", "test_request":
		return DurabilityLive
	default:
		return DurabilityCandidate
	}
}

func inferIngestKind(item IngestItem) string {
	switch item.Type {
	case "rejection", "preference":
		return "review_direction"
	case "review_direction":
		return "review_direction"
	case "requirement", "checklist", "test request", "test_request":
		return "review_requirement"
	default:
		return "review_direction"
	}
}

func ingestTemplate(pr, issue string, areas []string) string {
	areasBlock := ""
	for _, area := range areas {
		areasBlock += "- " + area + "\n"
	}
	if areasBlock == "" {
		areasBlock = "- review-area\n"
	}
	return `# Fabric PR Review Ingest

PR: ` + pr + `
Issue: ` + issue + `
Areas:
` + areasBlock + `
## Review directions

### Direction 1

Type: rejection
Durability: candidate

Review says:
Replace this with the review feedback that changes what another agent should do.

Rejected paths:
- A path the reviewer explicitly rejected
- Another path that should not be reopened

Preferred paths:
- The path the reviewer prefers
- Any specific implementation guidance

Reason:
Why the reviewer wants this direction. This helps future agents understand intent, not just instruction.

Evidence:
- reviewer comment: "Paste the relevant review comment here."

### Direction 2

Type: requirement
Durability: live

Review says:
Add tests for the ingest parser.

Reason:
The parser should not silently accept malformed review-ingest files.
`
}
