package fabric

import "testing"

func TestDirectionPacketSyncsAcrossThreads(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "VS-123", "--area", "virtual-store/listing")
	mustRun(t, "thread", "start", "--id", "thread-b", "--issue", "VS-123", "--area", "virtual-store/listing")
	mustRun(t, "note", "--thread", "thread-a", "--issue", "VS-123", "--area", "virtual-store/listing", "Don't create a second listing endpoint; extend the existing one or escalate API direction")

	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ID != "evt_000001" {
		t.Fatalf("events = %#v, want one evt_000001", events)
	}

	threads, err := loadThreads()
	if err != nil {
		t.Fatal(err)
	}
	if got := threads["thread-a"].LastSeenEventID; got != "evt_000001" {
		t.Fatalf("thread-a last seen = %q, want evt_000001", got)
	}
	if got := threads["thread-b"].LastSeenEventID; got != "" {
		t.Fatalf("thread-b last seen before sync = %q, want empty", got)
	}

	mustRun(t, "sync", "--thread", "thread-b", "--budget", "300")
	threads, err = loadThreads()
	if err != nil {
		t.Fatal(err)
	}
	if got := threads["thread-b"].LastSeenEventID; got != "evt_000001" {
		t.Fatalf("thread-b last seen after sync = %q, want evt_000001", got)
	}

	syncDelta := mustRead(t, syncPath)
	assertContains(t, syncDelta, "Don't create a second listing endpoint")
	assertContains(t, syncDelta, "Human note from related thread thread-a.")
	assertContains(t, syncDelta, "- Same issue: VS-123")
	assertContains(t, syncDelta, "- Same area: virtual-store/listing")
}

func TestPreflightAcceptsTaskBeforeFlags(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "note", "--global", "Prefer the existing extension points before adding new surfaces")
	mustRun(t, "preflight", "add filtering to virtual-store listing", "--issue", "VS-123", "--area", "virtual-store/listing", "--budget", "800")

	taskDirection := mustRead(t, taskPath)
	assertContains(t, taskDirection, "Task:\nadd filtering to virtual-store listing")
	assertContains(t, taskDirection, "Prefer the existing extension points")
}
