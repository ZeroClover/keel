package kubernetes

import (
	"testing"

	"github.com/keel-hq/keel/internal/k8s"
	"github.com/keel-hq/keel/internal/policy"
	"github.com/keel-hq/keel/types"

	apps_v1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCheckForUpdateAllowsSemVerEvent(t *testing.T) {
	plc, err := policy.NewSemVer(">=0.0.0")
	if err != nil {
		t.Fatal(err)
	}
	resource := deploymentWithContainer("gcr.io/v2-namespace/hello-world:1.0.0")

	plan, update, err := checkForUpdate(plc, nil, &types.Repository{Name: "gcr.io/v2-namespace/hello-world", Tag: "1.1.0"}, resource)
	if err != nil {
		t.Fatal(err)
	}
	if !update {
		t.Fatal("expected update")
	}
	if plan.CurrentVersion != "1.0.0" || plan.NewVersion != "1.1.0" {
		t.Fatalf("plan version = %s -> %s", plan.CurrentVersion, plan.NewVersion)
	}
	if got := resource.Containers()[0].Image; got != "gcr.io/v2-namespace/hello-world:1.1.0" {
		t.Fatalf("container image = %q", got)
	}
}

func TestCheckForUpdateRejectsDifferentRepository(t *testing.T) {
	resource := deploymentWithContainer("gcr.io/v2-namespace/goodbye-world:1.0.0")

	plan, update, err := checkForUpdate(policy.NewForce(), nil, &types.Repository{Name: "gcr.io/v2-namespace/hello-world", Tag: "latest"}, resource)
	if err != nil {
		t.Fatal(err)
	}
	if update {
		t.Fatal("expected no update")
	}
	if plan.Resource != nil {
		t.Fatalf("expected empty plan, got %#v", plan)
	}
}

func TestCheckForUpdateRejectsFilteredEventTag(t *testing.T) {
	filter, err := policy.NewRegexFilter(`^release-.*$`, "")
	if err != nil {
		t.Fatal(err)
	}
	resource := deploymentWithContainer("gcr.io/v2-namespace/hello-world:release-1")

	_, update, err := checkForUpdate(policy.NewForce(), filter, &types.Repository{Name: "gcr.io/v2-namespace/hello-world", Tag: "master"}, resource)
	if err != nil {
		t.Fatal(err)
	}
	if update {
		t.Fatal("expected filter to reject event tag")
	}
}

func TestCheckForUpdateUpdatesInitContainer(t *testing.T) {
	resource := MustParseGR(&apps_v1.Deployment{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:        "dep-1",
			Namespace:   "default",
			Annotations: map[string]string{types.KeelInitContainerAnnotation: "true"},
		},
		Spec: apps_v1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: meta_v1.ObjectMeta{Annotations: map[string]string{}},
				Spec: v1.PodSpec{
					InitContainers: []v1.Container{{Image: "gcr.io/v2-namespace/hello-world:old"}},
					Containers:     []v1.Container{{Image: "gcr.io/v2-namespace/sidecar:old"}},
				},
			},
		},
	})

	plan, update, err := checkForUpdate(policy.NewForce(), nil, &types.Repository{Name: "gcr.io/v2-namespace/hello-world", Tag: "new"}, resource)
	if err != nil {
		t.Fatal(err)
	}
	if !update {
		t.Fatal("expected update")
	}
	if plan.CurrentVersion != "old" || plan.NewVersion != "new" {
		t.Fatalf("plan version = %s -> %s", plan.CurrentVersion, plan.NewVersion)
	}
	if got := resource.InitContainers()[0].Image; got != "gcr.io/v2-namespace/hello-world:new" {
		t.Fatalf("init container image = %q", got)
	}
}

func deploymentWithContainer(image string) *k8s.GenericResource {
	return MustParseGR(&apps_v1.Deployment{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:        "dep-1",
			Namespace:   "default",
			Annotations: map[string]string{},
		},
		Spec: apps_v1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: meta_v1.ObjectMeta{Annotations: map[string]string{}},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{Image: image}},
				},
			},
		},
	})
}
