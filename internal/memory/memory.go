package memory

import (
	"fmt"
	"strings"
)

type Manager struct {
	store    Store
	maxFacts int
	data     Data
}

func NewManager(path string, maxFacts int) *Manager {
	if maxFacts <= 0 {
		maxFacts = 50
	}
	return &Manager{store: Store{Path: path}, maxFacts: maxFacts, data: Data{Facts: []string{}}}
}

func (m *Manager) Load() (bool, error) {
	data, err := m.store.Load()
	if err != nil {
		if err.Error() == "corrupted" {
			m.data = Data{Facts: []string{}}
			return true, nil
		}
		return false, err
	}
	m.data = data
	return false, nil
}

func (m *Manager) Save() error {
	return m.store.Save(m.data)
}

func (m *Manager) Reset() error {
	m.data = Data{Facts: []string{}, Summary: ""}
	return m.store.Reset()
}

func (m *Manager) Facts() []string {
	out := make([]string, len(m.data.Facts))
	copy(out, m.data.Facts)
	return out
}

func (m *Manager) Summary() string {
	return m.data.Summary
}

func (m *Manager) SetSummary(summary string) {
	m.data.Summary = strings.TrimSpace(summary)
}

func (m *Manager) SetFacts(facts []string) {
	filtered := make([]string, 0, len(facts))
	for _, f := range facts {
		f = cleanFact(f)
		if f == "" {
			continue
		}
		filtered = append(filtered, f)
	}
	if len(filtered) > m.maxFacts {
		filtered = filtered[len(filtered)-m.maxFacts:]
	}
	m.data.Facts = filtered
}

func (m *Manager) AddFact(f string) {
	f = cleanFact(f)
	if f == "" {
		return
	}
	m.data.Facts = append(m.data.Facts, f)
	if len(m.data.Facts) > m.maxFacts {
		m.data.Facts = m.data.Facts[len(m.data.Facts)-m.maxFacts:]
	}
}

func (m *Manager) InjectionMessage() string {
	facts := "none"
	if len(m.data.Facts) > 0 {
		facts = strings.Join(m.data.Facts, ", ")
	}
	summary := m.data.Summary
	if summary == "" {
		summary = "none"
	}
	return fmt.Sprintf("[memory] You know these facts about the user: %s. Recent context: %s", facts, summary)
}

func cleanFact(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	words := strings.Fields(s)
	if len(words) > 20 {
		words = words[:20]
	}
	return strings.Join(words, " ")
}
