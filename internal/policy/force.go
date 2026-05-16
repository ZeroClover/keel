package policy

import "github.com/keel-hq/keel/types"

type Force struct{}

func NewForce() *Force {
	return &Force{}
}

func (p *Force) Name() string {
	return "force"
}

func (p *Force) Type() types.PolicyType {
	return types.PolicyTypeForce
}

func (p *Force) Latest(candidates []string) (string, error) {
	if len(candidates) == 0 {
		return "", errEmpty
	}
	return candidates[0], nil
}
