package policy

import (
	"fmt"
	"strconv"

	"github.com/keel-hq/keel/types"
)

const (
	NumericalOrderAsc  = "asc"
	NumericalOrderDesc = "desc"
)

type Numerical struct {
	Order string
}

func NewNumerical(order string) (*Numerical, error) {
	switch order {
	case "":
		order = NumericalOrderAsc
	case NumericalOrderAsc, NumericalOrderDesc:
	default:
		return nil, fmt.Errorf("invalid numerical order %q", order)
	}
	return &Numerical{Order: order}, nil
}

func (p *Numerical) Name() string {
	return "numerical"
}

func (p *Numerical) Type() types.PolicyType {
	return types.PolicyTypeNumerical
}

func (p *Numerical) Latest(list []string) (string, error) {
	if len(list) == 0 {
		return "", errEmpty
	}

	latest := list[0]
	latestValue, err := strconv.ParseInt(latest, 10, 64)
	if err != nil {
		return "", fmt.Errorf("failed to parse numeric value %q: %w", latest, err)
	}

	for _, item := range list[1:] {
		value, err := strconv.ParseInt(item, 10, 64)
		if err != nil {
			return "", fmt.Errorf("failed to parse numeric value %q: %w", item, err)
		}

		if p.Order == NumericalOrderDesc {
			if value > latestValue {
				latest = item
				latestValue = value
			}
			continue
		}
		if value < latestValue {
			latest = item
			latestValue = value
		}
	}

	return latest, nil
}
