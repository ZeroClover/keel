package policy

import (
	"fmt"
	"regexp"
)

type RegexFilter struct {
	Regexp  *regexp.Regexp
	Replace string

	filtered map[string]string
	items    []string
}

func NewRegexFilter(pattern, replace string) (*RegexFilter, error) {
	m, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regular expression pattern %q: %w", pattern, err)
	}
	return &RegexFilter{
		Regexp:  m,
		Replace: replace,
	}, nil
}

func (f *RegexFilter) Apply(list []string) {
	f.filtered = map[string]string{}
	f.items = nil

	for _, item := range list {
		matches := f.Regexp.FindStringSubmatchIndex(item)
		if matches == nil {
			continue
		}

		key := item
		if f.Replace != "" {
			key = string(f.Regexp.ExpandString(nil, f.Replace, item, matches))
		}
		f.filtered[key] = item
		f.items = append(f.items, key)
	}
}

func (f *RegexFilter) Items() []string {
	return append([]string(nil), f.items...)
}

func (f *RegexFilter) GetOriginalTag(key string) string {
	return f.filtered[key]
}
