package fabric

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedAgentSkillsHaveMetadataAndFocusedWorkflows(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")

	expected := []string{
		"fabric-session",
		"fabric-record-direction",
		"fabric-pr-direction",
		"fabric-consolidate",
		"fabric-publish",
	}
	for _, name := range expected {
		content := mustRead(t, filepath.Join(".agents/skills", name, "SKILL.md"))
		if !strings.HasPrefix(content, "---\nname: "+name+"\ndescription: ") {
			t.Fatalf("%s does not start with valid skill frontmatter", name)
		}
		if !strings.Contains(content, "\n---\n") {
			t.Fatalf("%s does not close its frontmatter", name)
		}
		if strings.Contains(content, "TODO") {
			t.Fatalf("%s still contains TODO content", name)
		}
		metadata := mustRead(t, filepath.Join(".agents/skills", name, "agents/openai.yaml"))
		assertContains(t, metadata, "default_prompt:")
	}

	prSkill := mustRead(t, ".agents/skills/fabric-pr-direction/SKILL.md")
	assertContains(t, prSkill, "Prefer an available native GitHub connector")
	assertContains(t, prSkill, "--dry-run")
	assertContains(t, prSkill, "Do not ingest before the user selects items")
	assertContains(t, mustRead(t, ".agents/skills/fabric-pr-direction/references/github-acquisition.md"), "gh auth status")
	assertContains(t, mustRead(t, ".agents/skills/fabric-pr-direction/references/github-acquisition.md"), "gh api --paginate")
	assertContains(t, mustRead(t, ".agents/skills/fabric-publish/agents/openai.yaml"), "allow_implicit_invocation: false")
}

func TestInstallAgentsPreservesUnrelatedSkills(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")

	repoManagedPath := ".agents/skills/fabric-session/SKILL.md"
	if err := os.WriteFile(repoManagedPath, []byte("stale managed skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	globalRoot, err := globalAgentSkillsRoot()
	if err != nil {
		t.Fatal(err)
	}
	globalManagedPath := filepath.Join(globalRoot, "fabric-session", "SKILL.md")
	globalCustomPath := filepath.Join(globalRoot, "team-workflow", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(globalCustomPath), 0o755); err != nil {
		t.Fatal(err)
	}
	custom := "---\nname: team-workflow\ndescription: Team-owned workflow.\n---\n\nDo not replace me.\n"
	if err := os.WriteFile(globalCustomPath, []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("AGENTS.md", []byte("# Team instructions\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		mustRun(t, "install-agents")
	})

	if got := mustRead(t, globalCustomPath); got != custom {
		t.Fatalf("custom skill changed:\n%s", got)
	}
	assertContains(t, mustRead(t, repoManagedPath), "name: fabric-session")
	assertContains(t, mustRead(t, globalManagedPath), "name: fabric-session")
	assertContains(t, output, "Installed Direction Fabric skills globally in "+globalRoot)
	assertContains(t, mustRead(t, "AGENTS.md"), "# Team instructions")
	assertContains(t, mustRead(t, "AGENTS.md"), fabricBlockStart)
}

type skillTriggerEval struct {
	ID               string   `json:"id"`
	Prompt           string   `json:"prompt"`
	ExpectedSkill    string   `json:"expected_skill"`
	ExpectedOutcomes []string `json:"expected_outcomes"`
	ExternalWrite    bool     `json:"external_write"`
}

func TestSkillTriggerEvalCorpus(t *testing.T) {
	path := filepath.Join("..", "..", "evals", "skill-triggers.jsonl")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	known := map[string]bool{}
	for _, dir := range agentSkillDirs() {
		parts := strings.Split(filepath.ToSlash(dir), "/")
		if len(parts) >= 1 {
			known[parts[0]] = true
		}
	}
	seen := map[string]bool{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var item skillTriggerEval
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			t.Fatal(err)
		}
		if item.ID == "" || item.Prompt == "" || item.ExpectedSkill == "" || len(item.ExpectedOutcomes) == 0 {
			t.Fatalf("incomplete trigger eval: %#v", item)
		}
		if seen[item.ID] {
			t.Fatalf("duplicate trigger eval ID %q", item.ID)
		}
		seen[item.ID] = true
		if !known[item.ExpectedSkill] {
			t.Fatalf("unknown expected skill %q", item.ExpectedSkill)
		}
		if item.ExternalWrite && item.ExpectedSkill != "fabric-publish" {
			t.Fatalf("external-write eval %q must route to fabric-publish", item.ID)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(seen) < 7 {
		t.Fatalf("trigger eval count = %d, want at least 7", len(seen))
	}
}
