package helm3

import (
	"testing"

	"github.com/keel-hq/keel/types"

	hapi_chart "helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
)

func TestCheckReleaseSemVerUpdate(t *testing.T) {
	chart := mustChart(t, `
image:
  repository: gcr.io/v2-namespace/hello-world
  tag: 1.1.0
keel:
  policy: semver:>=0.0.0
  trigger: poll
  images:
    - repository: image.repository
      tag: image.tag
`)

	plan, update, err := checkRelease(&types.Repository{Name: "gcr.io/v2-namespace/hello-world", Tag: "1.1.2"}, "default", "release-1", chart, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if !update {
		t.Fatal("expected update")
	}
	if plan.Values["image.tag"] != "1.1.2" {
		t.Fatalf("values = %#v", plan.Values)
	}
	if plan.CurrentVersion != "1.1.0" || plan.NewVersion != "1.1.2" {
		t.Fatalf("plan version = %s -> %s", plan.CurrentVersion, plan.NewVersion)
	}
}

func TestCheckReleaseForceWithReleaseNotes(t *testing.T) {
	chart := mustChart(t, `
image:
  repository: gcr.io/v2-namespace/hello-world
  tag: alpha
keel:
  policy: force
  trigger: poll
  images:
    - repository: image.repository
      tag: image.tag
      releaseNotes: https://github.com/keel-hq/keel/releases
`)

	plan, update, err := checkRelease(&types.Repository{Name: "gcr.io/v2-namespace/hello-world", Tag: "beta"}, "default", "release-1", chart, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if !update {
		t.Fatal("expected update")
	}
	if plan.ReleaseNotes[0] != "https://github.com/keel-hq/keel/releases" {
		t.Fatalf("release notes = %#v", plan.ReleaseNotes)
	}
}

func TestCheckReleaseRejectsFilteredEvent(t *testing.T) {
	chart := mustChart(t, `
image:
  repository: gcr.io/v2-namespace/hello-world
  tag: release-1
keel:
  policy: force
  filterTags: "^release-.*$"
  images:
    - repository: image.repository
      tag: image.tag
`)

	_, update, err := checkRelease(&types.Repository{Name: "gcr.io/v2-namespace/hello-world", Tag: "master"}, "default", "release-1", chart, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if update {
		t.Fatal("expected filter to reject event")
	}
}

func TestCheckReleaseNoKeelConfig(t *testing.T) {
	chart := mustChart(t, `
image:
  repository: gcr.io/v2-namespace/hello-world
  tag: 1.0.0
`)

	_, update, err := checkRelease(&types.Repository{Name: "gcr.io/v2-namespace/hello-world", Tag: "1.1.0"}, "default", "release-1", chart, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if update {
		t.Fatal("expected no update")
	}
}

func mustChart(t *testing.T, raw string) *hapi_chart.Chart {
	t.Helper()
	vals, err := chartutil.ReadValues([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return &hapi_chart.Chart{
		Values:   vals,
		Metadata: &hapi_chart.Metadata{Name: "app-x"},
	}
}
