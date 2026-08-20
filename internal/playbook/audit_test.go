package playbook_test

import (
	"testing"

	"github.com/sapliy/sapliy-core/internal/playbook"
)

func sampleEntry(action string) playbook.DecisionEntry {
	return playbook.DecisionEntry{
		PlaybookID:    "revenue_recovery",
		TenantID:      "tenant-1",
		Event:         playbook.PaymentFailed,
		Action:        action,
		Reason:        "test reason",
		PolicyApplied: "dunning-policy",
		AIReasoning:   "test ai reasoning",
		Confidence:    0.85,
		Actor:         "engine",
	}
}

func TestDecisionLog_HashChainIntegrity(t *testing.T) {
	log := playbook.NewDecisionLog()

	_, err := log.Append(sampleEntry("schedule_retry"))
	if err != nil {
		t.Fatalf("Append #1: %v", err)
	}
	_, err = log.Append(sampleEntry("escalate"))
	if err != nil {
		t.Fatalf("Append #2: %v", err)
	}
	_, err = log.Append(sampleEntry("send_reminder"))
	if err != nil {
		t.Fatalf("Append #3: %v", err)
	}

	entries := log.Entries()
	if len(entries) != 3 {
		t.Fatalf("Entries() length = %d, want 3", len(entries))
	}

	// Every entry carries an ID, timestamp and hash.
	for i, e := range entries {
		if e.ID == "" {
			t.Errorf("entry %d missing ID", i)
		}
		if e.CreatedAt.IsZero() {
			t.Errorf("entry %d missing CreatedAt", i)
		}
		if e.Hash == "" {
			t.Errorf("entry %d missing Hash", i)
		}
	}

	// Chain linkage: genesis anchors the first, each next links to previous.
	if entries[0].PrevHash != "genesis" {
		t.Errorf("entry 0 PrevHash = %q, want genesis", entries[0].PrevHash)
	}
	if entries[1].PrevHash != entries[0].Hash {
		t.Errorf("entry 1 PrevHash does not link to entry 0 hash")
	}
	if entries[2].PrevHash != entries[1].Hash {
		t.Errorf("entry 2 PrevHash does not link to entry 1 hash")
	}

	// Clean chain verifies both live and after reload.
	if err := log.Verify(); err != nil {
		t.Errorf("Verify() on live log: %v", err)
	}
	reloaded := playbook.NewDecisionLogFromEntries(entries)
	if err := reloaded.Verify(); err != nil {
		t.Errorf("Verify() on reloaded log: %v", err)
	}
}

func TestDecisionLog_TamperDetection(t *testing.T) {
	log := playbook.NewDecisionLog()
	_, _ = log.Append(sampleEntry("schedule_retry"))
	_, _ = log.Append(sampleEntry("escalate"))
	_, _ = log.Append(sampleEntry("send_reminder"))

	tampered := log.Entries()
	tampered[1].Action = "hacked" // mutating a copy does NOT affect the log

	// The live log must be unaffected by the external copy.
	if err := log.Verify(); err != nil {
		t.Fatalf("live log Verify() after external copy tamper: %v", err)
	}

	// A reloaded chain built from tampered data must fail verification.
	corrupted := playbook.NewDecisionLogFromEntries(tampered)
	if err := corrupted.Verify(); err == nil {
		t.Fatalf("Verify() should report tampering but returned nil")
	}
}

func TestDecisionLog_TamperHashLink(t *testing.T) {
	log := playbook.NewDecisionLog()
	_, _ = log.Append(sampleEntry("schedule_retry"))
	_, _ = log.Append(sampleEntry("escalate"))

	entries := log.Entries()
	entries[1].PrevHash = "forged" // break the chain link
	forged := playbook.NewDecisionLogFromEntries(entries)

	if err := forged.Verify(); err == nil {
		t.Fatalf("Verify() should report broken chain link but returned nil")
	}
}

func TestDecisionLog_EntriesAreDefensiveCopy(t *testing.T) {
	log := playbook.NewDecisionLog()
	_, _ = log.Append(sampleEntry("schedule_retry"))

	entries := log.Entries()
	entries[0].Action = "mutated"

	if got := log.Entries()[0].Action; got != "schedule_retry" {
		t.Errorf("internal chain mutated via Entries() copy: action = %q", got)
	}
}

func TestDecisionLog_ConcurrentAppend(t *testing.T) {
	log := playbook.NewDecisionLog()

	const goroutines = 8
	const perGoroutine = 25

	done := make(chan struct{}, goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for i := 0; i < perGoroutine; i++ {
				if _, err := log.Append(sampleEntry("schedule_retry")); err != nil {
					t.Errorf("concurrent Append: %v", err)
					return
				}
			}
		}()
	}
	for g := 0; g < goroutines; g++ {
		<-done
	}

	if got := len(log.Entries()); got != goroutines*perGoroutine {
		t.Errorf("Entries() length = %d, want %d", got, goroutines*perGoroutine)
	}
	if err := log.Verify(); err != nil {
		t.Errorf("Verify() after concurrent appends: %v", err)
	}
}
