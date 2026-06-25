package skills

import (
	"strings"
	"testing"
)

func TestDirsAndFilesIncludeManagedSkills(t *testing.T) {
	dirs := Dirs()
	files := Files()

	for _, want := range []string{
		"fabric-recall/agents",
		"fabric-session/agents",
		"fabric-provenance/agents",
		"fabric-record-direction/agents",
		"fabric-pr-direction/agents",
		"fabric-pr-direction/references",
		"fabric-consolidate/agents",
		"fabric-publish/agents",
	} {
		if !containsString(dirs, want) {
			t.Fatalf("Dirs() missing %q from %v", want, dirs)
		}
	}

	byPath := map[string]string{}
	for _, file := range files {
		if file.Path == "" || file.Content == "" {
			t.Fatalf("empty file entry: %#v", file)
		}
		byPath[file.Path] = file.Content
	}
	for _, want := range []string{
		"fabric-recall/SKILL.md",
		"fabric-session/SKILL.md",
		"fabric-provenance/SKILL.md",
		"fabric-record-direction/SKILL.md",
		"fabric-pr-direction/SKILL.md",
		"fabric-pr-direction/references/github-acquisition.md",
		"fabric-consolidate/SKILL.md",
		"fabric-publish/SKILL.md",
	} {
		if byPath[want] == "" {
			t.Fatalf("Files() missing %q", want)
		}
	}

	if !strings.Contains(byPath["fabric-session/SKILL.md"], "fabric status once") {
		t.Fatal("fabric-session skill lost status guidance")
	}
	if !strings.Contains(byPath["fabric-pr-direction/references/github-acquisition.md"], "gh pr view") {
		t.Fatal("github acquisition reference lost gh guidance")
	}
}

func TestRootAgentsProtocolHelpers(t *testing.T) {
	protocol := RootAgentsProtocol()
	if !strings.Contains(protocol, "$fabric-session") {
		t.Fatal("root protocol missing fabric-session guidance")
	}
	if AgentsSnippet() != protocol {
		t.Fatal("AgentsSnippet should mirror RootAgentsProtocol")
	}

	block := RootAgentsBlock()
	if !strings.HasPrefix(block, "<!-- fabric:start -->\n") || !strings.HasSuffix(block, "<!-- fabric:end -->\n") {
		t.Fatalf("RootAgentsBlock markers malformed: %q", block)
	}
	if !strings.Contains(block, protocol) {
		t.Fatal("RootAgentsBlock missing root protocol")
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
