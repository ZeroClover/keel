package helm3

import (
	"testing"

	"github.com/keel-hq/keel/types"

	hapi_chart "helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
)

func TestCheckReleaseWebhookRejectsSemVerOutsideRange(t *testing.T) {
	chart := chartWithPolicy(t, "semver:>=1.25.0, <1.26.0", "1.25.3", "")

	_, update, err := checkRelease(repo("gcr.io/v2-namespace/hello-world", "1.26.0"), "native", "default", "release-1", chart, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if update {
		t.Fatal("expected semver range to reject webhook tag")
	}
}

func TestCheckReleaseWebhookAllowsSemVerUpgrade(t *testing.T) {
	chart := chartWithPolicy(t, "semver:>=1.25.0, <1.26.0", "1.25.3", "")

	plan, update, err := checkRelease(repo("gcr.io/v2-namespace/hello-world", "1.25.4"), "native", "default", "release-1", chart, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if !update {
		t.Fatal("expected update")
	}
	if plan.Values["image.tag"] != "1.25.4" {
		t.Fatalf("values = %#v", plan.Values)
	}
	if plan.CurrentVersion != "1.25.3" || plan.NewVersion != "1.25.4" {
		t.Fatalf("plan version = %s -> %s", plan.CurrentVersion, plan.NewVersion)
	}
}

func TestCheckReleaseWebhookRejectsSemVerDowngrade(t *testing.T) {
	chart := chartWithPolicy(t, "semver:>=1.25.0, <1.26.0", "1.25.3", "")

	_, update, err := checkRelease(repo("gcr.io/v2-namespace/hello-world", "1.25.2"), "native", "default", "release-1", chart, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if update {
		t.Fatal("expected downgrade to be rejected")
	}
}

func TestCheckReleaseWebhookRejectsFilteredEventTag(t *testing.T) {
	chart := chartWithPolicy(t, "numerical:desc", "main-100", `  filterTags: "^main-(?P<n>[0-9]+)$"
  extract: "$n"
`)

	_, update, err := checkRelease(repo("gcr.io/v2-namespace/hello-world", "prod-200"), "native", "default", "release-1", chart, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if update {
		t.Fatal("expected filter to reject event tag")
	}
}

func TestCheckReleaseCurrentTagFilteredOutStillAllowsEventTag(t *testing.T) {
	chart := chartWithPolicy(t, "numerical:desc", "legacy-100", `  filterTags: "^main-(?P<n>[0-9]+)$"
  extract: "$n"
`)

	_, update, err := checkRelease(repo("gcr.io/v2-namespace/hello-world", "main-200"), "native", "default", "release-1", chart, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if !update {
		t.Fatal("expected admitted event tag to update")
	}
}

func TestCheckReleaseForceWebhookRespectsFilter(t *testing.T) {
	chart := chartWithPolicy(t, "force", "release-1", `  filterTags: "^release-[0-9]+$"
`)

	_, update, err := checkRelease(repo("gcr.io/v2-namespace/hello-world", "dev-2"), "native", "default", "release-1", chart, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if update {
		t.Fatal("expected filter to reject force webhook tag")
	}

	_, update, err = checkRelease(repo("gcr.io/v2-namespace/hello-world", "release-2"), "native", "default", "release-1", chart, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if !update {
		t.Fatal("expected filter-admitted force webhook tag to update")
	}
}

func TestCheckReleaseNonPollTriggerNamesUseExternalAdmission(t *testing.T) {
	for _, triggerName := range []string{"pubsub", "", "unknown"} {
		t.Run(triggerName, func(t *testing.T) {
			chart := chartWithPolicy(t, "semver:>=1.25.0, <1.26.0", "1.25.3", "")

			_, update, err := checkRelease(repo("gcr.io/v2-namespace/hello-world", "1.26.0"), triggerName, "default", "release-1", chart, map[string]interface{}{})
			if err != nil {
				t.Fatal(err)
			}
			if update {
				t.Fatal("expected non-poll trigger to require policy admission")
			}
		})
	}
}

func TestCheckReleasePollTrustsWatcherSelectedTag(t *testing.T) {
	chart := chartWithPolicy(t, "semver:>=1.25.0, <1.26.0", "1.25.3", "")

	_, update, err := checkRelease(repo("gcr.io/v2-namespace/hello-world", "1.26.0"), types.TriggerTypePoll.String(), "default", "release-1", chart, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if !update {
		t.Fatal("expected poll event to update without re-running policy")
	}
}

func TestCheckReleaseSameTagNoop(t *testing.T) {
	chart := chartWithPolicy(t, "force", "1.25.3", "")

	_, update, err := checkRelease(repo("gcr.io/v2-namespace/hello-world", "1.25.3"), "native", "default", "release-1", chart, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if update {
		t.Fatal("expected no update")
	}
}

func TestCheckReleaseDifferentRepositoryNoop(t *testing.T) {
	chart := chartWithPolicy(t, "force", "1.25.3", "")

	_, update, err := checkRelease(repo("gcr.io/v2-namespace/goodbye-world", "1.25.4"), "native", "default", "release-1", chart, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if update {
		t.Fatal("expected no update")
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

	plan, update, err := checkRelease(repo("gcr.io/v2-namespace/hello-world", "beta"), types.TriggerTypePoll.String(), "default", "release-1", chart, map[string]interface{}{})
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

func TestCheckReleaseNoKeelConfig(t *testing.T) {
	chart := mustChart(t, `
image:
  repository: gcr.io/v2-namespace/hello-world
  tag: 1.0.0
`)

	_, update, err := checkRelease(repo("gcr.io/v2-namespace/hello-world", "1.1.0"), "native", "default", "release-1", chart, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if update {
		t.Fatal("expected no update")
	}
}

func repo(name, tag string) *types.Repository {
	return &types.Repository{Name: name, Tag: tag}
}

func chartWithPolicy(t *testing.T, policy, tag, extraKeel string) *hapi_chart.Chart {
	t.Helper()
	return mustChart(t, `
image:
  repository: gcr.io/v2-namespace/hello-world
  tag: `+tag+`
keel:
  policy: `+policy+`
  trigger: poll
`+extraKeel+`  images:
    - repository: image.repository
      tag: image.tag
`)
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
