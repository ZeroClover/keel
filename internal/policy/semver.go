package policy

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/keel-hq/keel/types"
)

type SemVer struct {
	Range      string
	constraint *semver.Constraints
}

func NewSemVer(r string) (*SemVer, error) {
	constraint, err := semver.NewConstraint(r)
	if err != nil {
		return nil, err
	}
	return &SemVer{
		Range:      r,
		constraint: constraint,
	}, nil
}

func (p *SemVer) Name() string {
	return "semver"
}

func (p *SemVer) Type() types.PolicyType {
	return types.PolicyTypeSemver
}

func (p *SemVer) Latest(versions []string) (string, error) {
	if len(versions) == 0 {
		return "", errEmpty
	}

	var latest *semver.Version
	for _, tag := range versions {
		v, err := semver.NewVersion(tag)
		if err != nil {
			continue
		}
		if p.constraint.Check(v) && (latest == nil || v.GreaterThan(latest)) {
			latest = v
		}
	}

	if latest == nil {
		return "", fmt.Errorf("%w", errNoMatch)
	}
	return latest.Original(), nil
}
