package filematch

import (
	"path/filepath"
	"strings"

	"github.com/gobwas/glob"
)

type Matcher struct {
	extensions  map[string]bool
	patterns    []glob.Glob
	languageIDs map[string]bool
}

func New(extensions, patterns, languageIDs []string) (*Matcher, error) {
	m := &Matcher{
		extensions:  make(map[string]bool),
		languageIDs: make(map[string]bool),
	}

	for _, ext := range extensions {
		normalized := strings.ToLower(ext)
		if !strings.HasPrefix(normalized, ".") {
			normalized = "." + normalized
		}
		m.extensions[normalized] = true
	}

	for _, pattern := range patterns {
		g, err := glob.Compile(pattern)
		if err != nil {
			return nil, err
		}
		m.patterns = append(m.patterns, g)
	}

	for _, langID := range languageIDs {
		m.languageIDs[strings.ToLower(langID)] = true
	}

	return m, nil
}

func (m *Matcher) MatchesExtension(ext string) bool {
	if len(m.extensions) == 0 {
		return false
	}
	normalized := strings.ToLower(ext)
	if !strings.HasPrefix(normalized, ".") {
		normalized = "." + normalized
	}
	return m.extensions[normalized]
}

func (m *Matcher) MatchesPattern(path string) bool {
	if len(m.patterns) == 0 {
		return false
	}
	filename := filepath.Base(path)
	for _, g := range m.patterns {
		if g.Match(filename) || g.Match(path) {
			return true
		}
	}
	return false
}

func (m *Matcher) MatchesLanguageID(langID string) bool {
	if len(m.languageIDs) == 0 {
		return false
	}
	return m.languageIDs[strings.ToLower(langID)]
}

func (m *Matcher) Matches(path, ext, languageID string) bool {
	if languageID != "" && m.MatchesLanguageID(languageID) {
		return true
	}

	if ext != "" && m.MatchesExtension(ext) {
		return true
	}

	if path != "" && m.MatchesPattern(path) {
		return true
	}

	return false
}

type MatcherSet struct {
	matchers []namedMatcher
}

type namedMatcher struct {
	name    string
	matcher *Matcher
}

func NewMatcherSet() *MatcherSet {
	return &MatcherSet{}
}

func (ms *MatcherSet) Add(name string, extensions, patterns, languageIDs []string) error {
	m, err := New(extensions, patterns, languageIDs)
	if err != nil {
		return err
	}
	ms.matchers = append(ms.matchers, namedMatcher{name: name, matcher: m})
	return nil
}

func (ms *MatcherSet) Match(path, ext, languageID string) string {
	for _, nm := range ms.matchers {
		if nm.matcher.Matches(path, ext, languageID) {
			return nm.name
		}
	}
	return ""
}

func (ms *MatcherSet) MatchByExtension(ext string) string {
	for _, nm := range ms.matchers {
		if nm.matcher.MatchesExtension(ext) {
			return nm.name
		}
	}
	return ""
}

func (ms *MatcherSet) MatchByLanguageID(langID string) string {
	for _, nm := range ms.matchers {
		if nm.matcher.MatchesLanguageID(langID) {
			return nm.name
		}
	}
	return ""
}
