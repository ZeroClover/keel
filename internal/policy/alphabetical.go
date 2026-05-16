package policy

import (
	"fmt"

	"github.com/keel-hq/keel/types"
)

const (
	AlphabeticalOrderAsc  = "asc"
	AlphabeticalOrderDesc = "desc"
)

type Alphabetical struct {
	Order string
}

func NewAlphabetical(order string) (*Alphabetical, error) {
	switch order {
	case "":
		order = AlphabeticalOrderAsc
	case AlphabeticalOrderAsc, AlphabeticalOrderDesc:
	default:
		return nil, fmt.Errorf("invalid alphabetical order %q", order)
	}
	return &Alphabetical{Order: order}, nil
}

func (p *Alphabetical) Name() string {
	return "alphabetical"
}

func (p *Alphabetical) Type() types.PolicyType {
	return types.PolicyTypeAlphabetical
}

func (p *Alphabetical) Latest(list []string) (string, error) {
	if len(list) == 0 {
		return "", errEmpty
	}

	latest := list[0]
	for _, item := range list[1:] {
		if p.Order == AlphabeticalOrderDesc {
			if item > latest {
				latest = item
			}
			continue
		}
		if item < latest {
			latest = item
		}
	}
	return latest, nil
}
