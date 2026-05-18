package policy

import (
	"errors"
	"fmt"
	"strings"

	"github.com/keel-hq/keel/types"
)

const (
	legacyPolicyLabel               = "keel.observer/policy"
	legacyForceTagMatchLabel        = "keel.sh/matchTag"
	legacyForceTagMatchLegacyLabel  = "keel.sh/match-tag"
	legacyMatchPreReleaseAnnotation = "keel.sh/matchPreRelease"
)

var (
	errUnsupportedPolicy = errors.New("unsupported legacy policy configuration")
	errEmpty             = errors.New("candidate list cannot be empty")
	errNoMatch           = errors.New("unable to determine latest candidate from provided list")
)

type Policy interface {
	Name() string
	Type() types.PolicyType
	Latest(candidates []string) (string, error)
}

type Filter interface {
	Apply(tags []string)
	Items() []string
	GetOriginalTag(key string) string
}

// GetPolicyFromLabelsOrAnnotations gets policy and filter configuration from Kubernetes metadata.
func GetPolicyFromLabelsOrAnnotations(labels map[string]string, annotations map[string]string) (Policy, Filter, error) {
	if legacyKey, ok := findLegacyPolicyKey(labels, annotations); ok {
		return nil, nil, unsupportedPolicyError("legacy policy key %q is no longer supported; use %q with semver/alphabetical/numerical/force syntax", legacyKey, types.KeelPolicyLabel)
	}
	if legacyKey, ok := findLegacyMatchKey(labels, annotations); ok {
		return nil, nil, unsupportedPolicyError("legacy annotation %q is no longer supported; use %q and %q instead", legacyKey, types.KeelFilterTagsAnnotation, types.KeelExtractAnnotation)
	}

	policyName := readPreferredValue(labels, annotations, types.KeelPolicyLabel)
	policyName = strings.TrimSpace(policyName)
	if policyName == "" || policyName == "never" {
		return nil, nil, nil
	}

	plc, err := parsePolicy(policyName)
	if err != nil {
		return nil, nil, err
	}

	var filter Filter
	filterTags := readPreferredValue(labels, annotations, types.KeelFilterTagsAnnotation)
	if filterTags != "" {
		filter, err = NewRegexFilter(filterTags, readPreferredValue(labels, annotations, types.KeelExtractAnnotation))
		if err != nil {
			return nil, nil, err
		}
	}

	return plc, filter, nil
}

func AllowsTag(plc types.Policy, filter types.Filter, currentTag, eventTag string) (bool, error) {
	if plc == nil || eventTag == "" {
		return false, nil
	}

	if plc.Type() == types.PolicyTypeForce {
		if filter == nil {
			return true, nil
		}
		filter.Apply([]string{eventTag})
		return len(filter.Items()) > 0, nil
	}

	candidates := []string{currentTag, eventTag}
	eventKey := eventTag
	if filter != nil {
		filter.Apply(candidates)
		candidates = filter.Items()
		foundEvent := false
		for _, key := range candidates {
			if filter.GetOriginalTag(key) == eventTag {
				eventKey = key
				foundEvent = true
				break
			}
		}
		if !foundEvent {
			return false, nil
		}
	}

	latestKey, err := plc.Latest(candidates)
	if err != nil {
		return false, err
	}

	if filter != nil {
		return filter.GetOriginalTag(latestKey) == eventTag, nil
	}
	return latestKey == eventKey, nil
}

func parsePolicy(policyName string) (Policy, error) {
	lower := strings.ToLower(policyName)
	if isLegacyPolicyValue(lower) {
		return nil, unsupportedPolicyError("legacy policy value %q is no longer supported", policyName)
	}

	switch {
	case strings.HasPrefix(policyName, "semver:"):
		constraint := strings.TrimPrefix(policyName, "semver:")
		if strings.TrimSpace(constraint) == "" {
			return nil, fmt.Errorf("semver policy requires a constraint")
		}
		return NewSemVer(constraint)
	case lower == "force" || strings.HasPrefix(lower, "force:"):
		return NewForceWithOption(policyOrder(lower, "force"))
	case lower == "alphabetical" || strings.HasPrefix(lower, "alphabetical:"):
		return NewAlphabetical(policyOrder(lower, "alphabetical"))
	case lower == "numerical" || strings.HasPrefix(lower, "numerical:"):
		return NewNumerical(policyOrder(lower, "numerical"))
	default:
		return nil, fmt.Errorf("unsupported policy %q", policyName)
	}
}

func policyOrder(policyName, prefix string) string {
	prefix += ":"
	if !strings.HasPrefix(policyName, prefix) {
		return ""
	}
	return strings.TrimPrefix(policyName, prefix)
}

func readPreferredValue(labels map[string]string, annotations map[string]string, key string) string {
	if value, ok := annotations[key]; ok {
		return value
	}
	return labels[key]
}

func findLegacyPolicyKey(labels map[string]string, annotations map[string]string) (string, bool) {
	if _, ok := annotations[types.KeelPolicyLabel]; ok {
		return "", false
	}
	if _, ok := labels[types.KeelPolicyLabel]; ok {
		return "", false
	}
	if _, ok := annotations[legacyPolicyLabel]; ok {
		return legacyPolicyLabel, true
	}
	if _, ok := labels[legacyPolicyLabel]; ok {
		return legacyPolicyLabel, true
	}
	return "", false
}

func findLegacyMatchKey(labels map[string]string, annotations map[string]string) (string, bool) {
	for _, metadata := range []map[string]string{annotations, labels} {
		for _, key := range []string{legacyForceTagMatchLabel, legacyForceTagMatchLegacyLabel, legacyMatchPreReleaseAnnotation} {
			if _, ok := metadata[key]; ok {
				return key, true
			}
		}
	}
	return "", false
}

func isLegacyPolicyValue(policyName string) bool {
	switch policyName {
	case "all", "major", "minor", "patch":
		return true
	}
	return strings.HasPrefix(policyName, "glob:") || strings.HasPrefix(policyName, "regexp:")
}

func unsupportedPolicyError(format string, args ...interface{}) error {
	return fmt.Errorf("%w: %s", errUnsupportedPolicy, fmt.Sprintf(format, args...))
}
