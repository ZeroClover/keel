package policy

import (
	"fmt"

	"github.com/keel-hq/keel/types"
)

// ForceOptionCreated enables sorting candidate tags by their image config
// `created` time before selecting the latest one. It requires an extra
// registry API call per tag and can be slow / rate-limited on large
// repositories, so it is opt-in.
const ForceOptionCreated = "created"

type Force struct {
	sortByCreated bool
}

func NewForce() *Force {
	return &Force{}
}

// NewForceWithOption builds a Force policy with the supplied option.
// Supported values:
//   - "" (default): pick the first candidate as returned by the registry.
//   - "created":    fetch each tag's image config `created` time and sort
//     descending before selecting the newest tag.
func NewForceWithOption(option string) (*Force, error) {
	switch option {
	case "":
		return &Force{}, nil
	case ForceOptionCreated:
		return &Force{sortByCreated: true}, nil
	default:
		return nil, fmt.Errorf("invalid force option %q", option)
	}
}

func (p *Force) Name() string {
	return "force"
}

func (p *Force) Type() types.PolicyType {
	return types.PolicyTypeForce
}

// SortByCreated reports whether candidate tags should be sorted by their
// image config `created` time before picking the latest one.
func (p *Force) SortByCreated() bool {
	return p.sortByCreated
}

func (p *Force) Latest(candidates []string) (string, error) {
	if len(candidates) == 0 {
		return "", errEmpty
	}
	return candidates[0], nil
}
