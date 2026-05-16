package helm3

import (
	"testing"

	"github.com/keel-hq/keel/extension/notification"
	"github.com/keel-hq/keel/types"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
)

type fakeSender struct {
	sentEvent types.EventNotification
}

func (s *fakeSender) Configure(cfg *notification.Config) (bool, error) {
	return true, nil
}

func (s *fakeSender) Send(event types.EventNotification) error {
	s.sentEvent = event
	return nil
}

type fakeImplementer struct {
	listReleasesResponse []*release.Release

	updatedRlsName string
	updatedChart   *chart.Chart
	updatedValues  map[string]string
}

func (i *fakeImplementer) ListReleases() ([]*release.Release, error) {
	return i.listReleasesResponse, nil
}

func (i *fakeImplementer) UpdateReleaseFromChart(rlsName string, chart *chart.Chart, vals map[string]string, namespace string, opts ...bool) (*release.Release, error) {
	i.updatedRlsName = rlsName
	i.updatedChart = chart
	i.updatedValues = vals
	return &release.Release{Name: rlsName, Chart: chart, Version: 2}, nil
}

func testingConfigYaml(cfg *KeelChartConfig) (chartutil.Values, error) {
	root := &Root{Keel: *cfg}
	bts, err := yaml.Marshal(root)
	if err != nil {
		return nil, err
	}
	return chartutil.ReadValues(bts)
}

func testingStringToChart(raw string) (*chart.Chart, error) {
	chartVals, err := chartutil.ReadValues([]byte(raw))
	if err != nil {
		return nil, err
	}
	return &chart.Chart{
		Values:   chartVals,
		Metadata: &chart.Metadata{Name: "app-x"},
	}, nil
}

func TestGetKeelConfigPolicyAndFilter(t *testing.T) {
	vals, err := testingConfigYaml(&KeelChartConfig{
		Policy:     "numerical:desc",
		FilterTags: "^main-[a-f0-9]+-(?P<ts>[0-9]+)$",
		Extract:    "$ts",
		Trigger:    types.TriggerTypePoll,
		Images: []ImageDetails{
			{RepositoryPath: "image.repository", TagPath: "image.tag"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := getKeelConfig(vals)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Plc == nil || cfg.Plc.Type() != types.PolicyTypeNumerical {
		t.Fatalf("policy = %#v", cfg.Plc)
	}
	if cfg.Filter == nil {
		t.Fatal("expected filter")
	}
	if cfg.Trigger != types.TriggerTypePoll {
		t.Fatalf("trigger = %v", cfg.Trigger)
	}
}

func TestGetKeelConfigRejectsLegacyMatchTag(t *testing.T) {
	vals, err := chartutil.ReadValues([]byte(`
keel:
  policy: semver:^1.0
  matchTag: true
`))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := getKeelConfig(vals); err == nil {
		t.Fatal("expected legacy matchTag to fail policy parsing")
	}
}

func TestTrackedImages(t *testing.T) {
	myChart, err := testingStringToChart(`
image:
  repository: gcr.io/v2-namespace/bye-world
  tag: 1.1.0
keel:
  policy: semver:>=0.0.0
  trigger: poll
  images:
    - repository: image.repository
      tag: image.tag
`)
	if err != nil {
		t.Fatal(err)
	}

	prov := NewProvider(&fakeImplementer{
		listReleasesResponse: []*release.Release{{
			Name:      "release-1",
			Namespace: "default",
			Chart:     myChart,
			Config:    map[string]interface{}{},
		}},
	}, &fakeSender{})

	tracked, err := prov.TrackedImages()
	if err != nil {
		t.Fatal(err)
	}
	if len(tracked) != 1 {
		t.Fatalf("len(tracked) = %d, want 1", len(tracked))
	}
	if tracked[0].Image.Remote() != "gcr.io/v2-namespace/bye-world:1.1.0" {
		t.Fatalf("image = %s", tracked[0].Image.Remote())
	}
}

func TestProcessEventUpdatesRelease(t *testing.T) {
	myChart, err := testingStringToChart(`
image:
  repository: karolisr/webhook-demo
  tag: 0.0.10
keel:
  policy: semver:>=0.0.0
  trigger: poll
  images:
    - repository: image.repository
      tag: image.tag
`)
	if err != nil {
		t.Fatal(err)
	}

	impl := &fakeImplementer{
		listReleasesResponse: []*release.Release{{
			Name:      "release-1",
			Namespace: "default",
			Chart:     myChart,
			Config:    map[string]interface{}{},
		}},
	}
	provider := NewProvider(impl, &fakeSender{})

	err = provider.processEvent(&types.Event{
		Repository: types.Repository{Name: "karolisr/webhook-demo", Tag: "0.0.11"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if impl.updatedChart != myChart {
		t.Fatal("wrong chart updated")
	}
	if impl.updatedRlsName != "release-1" {
		t.Fatalf("release = %s", impl.updatedRlsName)
	}
	if impl.updatedValues["image.tag"] != "0.0.11" {
		t.Fatalf("updated values = %#v", impl.updatedValues)
	}
}
