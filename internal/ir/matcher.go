package ir

import (
	"crypto/sha256"
	"encoding/binary"
	"sort"
	"strings"
)

// IRMatch is the result of matching a command against the IR catalog.
type IRMatch struct {
	Domain       string
	CapabilityID string
	Params       []string // parameter spec from the capability
	VerbLen      int      // number of words in the matched verb
}

// ParamShape returns a normalized string representing the structural shape
// of this match. Used for NodeID generation — commands with same capability
// and same number of remaining args produce the same shape.
func (m *IRMatch) ParamShape() string {
	return m.CapabilityID
}

// IRNodeID computes a stable hash for an IR match, analogous to trace.NodeID.
func (m *IRMatch) IRNodeID() uint64 {
	h := sha256.New()
	h.Write([]byte("IR"))
	h.Write([]byte{0})
	h.Write([]byte(m.CapabilityID))
	h.Write([]byte{0})
	sum := h.Sum(nil)
	return binary.BigEndian.Uint64(sum[:8])
}

// verbEntry is a precomputed verb → capability mapping, sorted by verb length descending.
type verbEntry struct {
	verb   string   // e.g. "kubectl get pods"
	words  []string // split verb
	capID  string
	domain string
	params []string
}

// Matcher matches preprocessed Bash commands against the IR catalog.
type Matcher struct {
	entries []verbEntry // sorted longest-first for greedy matching
}

// NewMatcher builds a Matcher from a catalog. It precomputes a sorted verb
// index for efficient longest-prefix matching.
func NewMatcher(cat *Catalog) *Matcher {
	var entries []verbEntry
	for domain, caps := range cat.Domains {
		for capID, cap := range caps {
			for _, verb := range cap.Verbs {
				words := strings.Fields(verb)
				entries = append(entries, verbEntry{
					verb:   verb,
					words:  words,
					capID:  capID,
					domain: domain,
					params: cap.Params,
				})
			}
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return len(entries[i].words) > len(entries[j].words)
	})

	return &Matcher{entries: entries}
}

// Match tries to match a preprocessed command against the catalog.
// Returns nil if no verb matches.
func (m *Matcher) Match(command string) *IRMatch {
	words := strings.Fields(command)
	if len(words) == 0 {
		return nil
	}

	for _, entry := range m.entries {
		if len(entry.words) > len(words) {
			continue
		}
		match := true
		for i, w := range entry.words {
			if !strings.EqualFold(words[i], w) {
				match = false
				break
			}
		}
		if match {
			return &IRMatch{
				Domain:       entry.domain,
				CapabilityID: entry.capID,
				Params:       entry.params,
				VerbLen:      len(entry.words),
			}
		}
	}

	return nil
}
