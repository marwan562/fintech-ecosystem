package playbook

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// genesisHash anchors the first entry of every decision log chain.
const genesisHash = "genesis"

// DecisionEntry is a single audited decision recorded by the playbook engine.
// PrevHash links to the previous entry and Hash is the SHA-256 of
// (prevHash + canonical JSON of the entry), forming an immutable chain.
type DecisionEntry struct {
	ID            string             `json:"id"`
	PlaybookID    string             `json:"playbookId"`
	TenantID      string             `json:"tenantId"`
	Event         FinancialEventType `json:"event"`
	Action        string             `json:"action"`
	Reason        string             `json:"reason"`
	PolicyApplied string             `json:"policyApplied"`
	AIReasoning   string             `json:"aiReasoning,omitempty"`
	Confidence    float64            `json:"confidence"`
	Actor         string             `json:"actor,omitempty"`
	CreatedAt     time.Time          `json:"createdAt"`
	PrevHash      string             `json:"prevHash,omitempty"`
	Hash          string             `json:"hash"`
}

// DecisionLog is an append-only, hash-chained decision log. It is safe for
// concurrent use.
type DecisionLog struct {
	mu    sync.Mutex
	chain []DecisionEntry
}

// NewDecisionLog creates an empty decision log.
func NewDecisionLog() *DecisionLog {
	return &DecisionLog{}
}

// NewDecisionLogFromEntries reconstructs a log from previously recorded
// entries, trusting their stored hashes. This is used to load a persisted
// chain so that Verify can detect tampering.
func NewDecisionLogFromEntries(entries []DecisionEntry) *DecisionLog {
	return &DecisionLog{chain: append([]DecisionEntry(nil), entries...)}
}

// Append records a decision entry, linking it to the previous entry's hash.
// An empty ID is assigned a UUID and a zero CreatedAt is set to now (UTC).
func (l *DecisionLog) Append(e DecisionEntry) (DecisionEntry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if e.ID == "" {
		e.ID = newID()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	} else {
		e.CreatedAt = e.CreatedAt.UTC()
	}

	prevHash := genesisHash
	if n := len(l.chain); n > 0 {
		prevHash = l.chain[n-1].Hash
	}
	e.PrevHash = prevHash

	hash, err := hashEntry(e, prevHash)
	if err != nil {
		return DecisionEntry{}, fmt.Errorf("failed to hash decision entry: %w", err)
	}
	e.Hash = hash

	l.chain = append(l.chain, e)
	return e, nil
}

// Entries returns a defensive copy of the chain, in append order.
func (l *DecisionLog) Entries() []DecisionEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]DecisionEntry(nil), l.chain...)
}

// Verify replays the chain and validates every hash link. It returns an error
// if any entry's hash or previous-hash link does not match.
func (l *DecisionLog) Verify() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	prevHash := genesisHash
	for i, e := range l.chain {
		if e.PrevHash != prevHash {
			return fmt.Errorf("decision log tampered: entry %d (%s) has prevHash %q, expected %q", i, e.ID, e.PrevHash, prevHash)
		}
		expected, err := hashEntry(e, prevHash)
		if err != nil {
			return fmt.Errorf("decision log verify failed at entry %d: %w", i, err)
		}
		if expected != e.Hash {
			return fmt.Errorf("decision log tampered: entry %d (%s) hash mismatch", i, e.ID)
		}
		prevHash = e.Hash
	}
	return nil
}

// hashEntry computes the SHA-256 of (prevHash + canonical JSON of the entry).
// The Hash field is excluded from the canonical JSON so the digest can be
// self-referential.
func hashEntry(e DecisionEntry, prevHash string) (string, error) {
	canonical := e
	canonical.Hash = ""
	data, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(append([]byte(prevHash), data...))
	return hex.EncodeToString(sum[:]), nil
}
