package helm3

import (
	"testing"

	"github.com/keel-hq/keel/types"
	"helm.sh/helm/v3/pkg/chartutil"
)

func TestGetImages(t *testing.T) {
	vals, err := chartutil.ReadValues([]byte(`
image:
  repository: gcr.io/v2-namespace/hello-world
  tag: 1.1.0
keel:
  policy: semver:>=0.0.0
  filterTags: "^v?(?P<v>.*)$"
  extract: "$v"
  trigger: poll
  images:
    - repository: image.repository
      tag: image.tag
      imagePullSecret: regcred
`))
	if err != nil {
		t.Fatal(err)
	}

	got, err := getImages(vals)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len(images) = %d, want 1", len(got))
	}
	if got[0].Image.Remote() != "gcr.io/v2-namespace/hello-world:1.1.0" {
		t.Fatalf("image = %s", got[0].Image.Remote())
	}
	if got[0].Trigger != types.TriggerTypePoll {
		t.Fatalf("trigger = %v", got[0].Trigger)
	}
	if got[0].Policy == nil || got[0].Policy.Type() != types.PolicyTypeSemver {
		t.Fatalf("policy = %#v", got[0].Policy)
	}
	if got[0].Filter == nil {
		t.Fatal("expected filter")
	}
	if len(got[0].Secrets) != 1 || got[0].Secrets[0] != "regcred" {
		t.Fatalf("secrets = %#v", got[0].Secrets)
	}
}

func TestGetImagesNoPolicy(t *testing.T) {
	vals, err := chartutil.ReadValues([]byte(`keel: {}`))
	if err != nil {
		t.Fatal(err)
	}

	got, err := getImages(vals)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("len(images) = %d, want 0", len(got))
	}
}
