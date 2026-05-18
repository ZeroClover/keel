package policy

import (
	"errors"
	"testing"

	"github.com/keel-hq/keel/types"
)

func TestGetPolicyFromLabelsOrAnnotations(t *testing.T) {
	tests := []struct {
		name        string
		labels      map[string]string
		annotations map[string]string
		wantType    types.PolicyType
		wantFilter  bool
		wantNil     bool
		wantErr     bool
	}{
		{
			name:        "semver annotation",
			annotations: map[string]string{types.KeelPolicyLabel: "semver:>=1.0.0-0"},
			wantType:    types.PolicyTypeSemver,
		},
		{
			name: "annotations override labels",
			labels: map[string]string{
				types.KeelPolicyLabel: "force",
			},
			annotations: map[string]string{
				types.KeelPolicyLabel: "alphabetical:asc",
			},
			wantType: types.PolicyTypeAlphabetical,
		},
		{
			name:        "alphabetical descending",
			annotations: map[string]string{types.KeelPolicyLabel: "alphabetical:desc"},
			wantType:    types.PolicyTypeAlphabetical,
		},
		{
			name:        "numerical descending",
			annotations: map[string]string{types.KeelPolicyLabel: "numerical:desc"},
			wantType:    types.PolicyTypeNumerical,
		},
		{
			name:        "force",
			annotations: map[string]string{types.KeelPolicyLabel: "force"},
			wantType:    types.PolicyTypeForce,
		},
		{
			name:        "force sort by created",
			annotations: map[string]string{types.KeelPolicyLabel: "force:created"},
			wantType:    types.PolicyTypeForce,
		},
		{
			name: "filter",
			annotations: map[string]string{
				types.KeelPolicyLabel:          "numerical:desc",
				types.KeelFilterTagsAnnotation: "^main-[a-f0-9]+-(?P<ts>[0-9]+)$",
				types.KeelExtractAnnotation:    "$ts",
			},
			wantType:   types.PolicyTypeNumerical,
			wantFilter: true,
		},
		{
			name:    "no policy",
			wantNil: true,
		},
		{
			name:        "never policy",
			annotations: map[string]string{types.KeelPolicyLabel: "never"},
			wantNil:     true,
		},
		{
			name:        "legacy policy value",
			annotations: map[string]string{types.KeelPolicyLabel: "minor"},
			wantErr:     true,
		},
		{
			name:        "legacy policy key",
			annotations: map[string]string{legacyPolicyLabel: "all"},
			wantErr:     true,
		},
		{
			name: "legacy match tag annotation",
			annotations: map[string]string{
				types.KeelPolicyLabel:    "force",
				legacyForceTagMatchLabel: "true",
			},
			wantErr: true,
		},
		{
			name: "legacy match prerelease annotation",
			annotations: map[string]string{
				types.KeelPolicyLabel:           "semver:^1",
				legacyMatchPreReleaseAnnotation: "false",
			},
			wantErr: true,
		},
		{
			name:        "legacy glob value",
			annotations: map[string]string{types.KeelPolicyLabel: "glob:release-*"},
			wantErr:     true,
		},
		{
			name:        "legacy regexp value",
			annotations: map[string]string{types.KeelPolicyLabel: "regexp:^build-([0-9]+)$"},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPolicy, gotFilter, err := GetPolicyFromLabelsOrAnnotations(tt.labels, tt.annotations)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if !errors.Is(err, errUnsupportedPolicy) {
					t.Fatalf("expected unsupported policy error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if gotPolicy != nil || gotFilter != nil {
					t.Fatalf("expected nil policy/filter, got %#v/%#v", gotPolicy, gotFilter)
				}
				return
			}
			if gotPolicy == nil {
				t.Fatal("expected policy")
			}
			if gotPolicy.Type() != tt.wantType {
				t.Fatalf("policy type = %v, want %v", gotPolicy.Type(), tt.wantType)
			}
			if (gotFilter != nil) != tt.wantFilter {
				t.Fatalf("filter present = %v, want %v", gotFilter != nil, tt.wantFilter)
			}
		})
	}
}

func TestAllowsTag(t *testing.T) {
	filter, err := NewRegexFilter("^build-(?P<n>[0-9]+)$", "$n")
	if err != nil {
		t.Fatal(err)
	}
	numerical, err := NewNumerical("desc")
	if err != nil {
		t.Fatal(err)
	}

	allowed, err := AllowsTag(numerical, filter, "build-10", "build-20")
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("expected newer extracted numerical tag to be allowed")
	}

	allowed, err = AllowsTag(numerical, filter, "build-20", "release-30")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("expected filtered event tag to be rejected")
	}

	force := NewForce()
	allowed, err = AllowsTag(force, filter, "build-20", "release-30")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("expected force policy to honor filterTags")
	}
}
