package cli

import "testing"

func TestChallengeFlowCreatesAndResolvesExplicitDirectionDispute(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "VS-123", "--area", "file-opening")
	mustRun(t, "note", "--durable", "--reason", "original product scope", "Do not implement full Office preview.")
	directionID := recordIDAt(t, 0)

	output := captureStdout(t, func() {
		mustRun(t, "challenge", "--direction", directionID, "--pr", "123", "--issue", "VS-123", "--area", "file-opening", "--proposal", "Implement internal Office preview", "--reason", "Product explicitly rescoped the work.")
	})
	challengeID := recordIDAt(t, 1)
	assertContains(t, output, "Recorded challenge "+challengeID+" against "+directionID)
	challenge := mustRead(t, challengePath)
	assertContains(t, challenge, "Challenged direction:\n"+directionID)

	mustRun(t, "continue", "--pr", "123", "--thread", "thread-c")
	assertContains(t, mustRead(t, continuePath), "Direction "+directionID+" is being challenged")

	mustRun(t, "challenge", "resolve", challengeID, "--accepted", "--reason", "Scoped exception approved")
	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	resolution := events[len(events)-1]
	if resolution.Kind != "challenge_resolution" || resolution.Challenges != challengeID || resolution.Status != "accepted" {
		t.Fatalf("resolution = %#v", resolution)
	}
	mustRun(t, "continue", "--pr", "123", "--thread", "thread-c")
	assertContains(t, mustRead(t, continuePath), "Scoped exception approved")
}

func TestChallengeValidationAndStatuses(t *testing.T) {
	chdirTemp(t)
	if err := Run([]string{"challenge", "--direction", "missing", "--issue", "VS-123", "proposal"}); err == nil {
		t.Fatal("challenge succeeded before init")
	}
	mustRun(t, "init")
	mustRun(t, "note", "--candidate", "--issue", "VS-123", "Original direction")
	directionID := recordIDAt(t, 0)

	if err := Run([]string{"challenge", "--direction", directionID, "--issue", "VS-123"}); err == nil {
		t.Fatal("challenge accepted empty proposal")
	}
	if err := Run([]string{"challenge", "--direction", "rec_missing", "--issue", "VS-123", "proposal"}); err == nil {
		t.Fatal("challenge accepted unknown direction")
	}
	mustRun(t, "challenge", "--direction", directionID, "--issue", "VS-123", "Challenge: use a new API.")
	challengeID := recordIDAt(t, 1)
	if err := Run([]string{"challenge", "resolve", challengeID}); err == nil {
		t.Fatal("resolve accepted no status")
	}
	if err := Run([]string{"challenge", "resolve", challengeID, "--accepted", "--rejected"}); err == nil {
		t.Fatal("resolve accepted multiple statuses")
	}
	mustRun(t, "challenge", "resolve", challengeID, "--superseded", "--reason", "New direction approved")
	if err := Run([]string{"challenge", "resolve", challengeID, "--accepted"}); err == nil {
		t.Fatal("challenge resolved twice")
	}
}

func TestChallengeCanBeRejected(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	mustRun(t, "note", "--candidate", "--issue", "VS-123", "Original direction")
	directionID := recordIDAt(t, 0)
	mustRun(t, "challenge", "--direction", directionID, "--issue", "VS-123", "Alternative")
	challengeID := recordIDAt(t, 1)
	mustRun(t, "challenge", "resolve", "--rejected", challengeID, "--reason", "Original direction still applies")
	events, _ := loadEvents()
	resolution := events[len(events)-1]
	if resolution.Status != "rejected" || resolution.Challenges != challengeID {
		t.Fatalf("resolution = %#v", resolution)
	}
}
