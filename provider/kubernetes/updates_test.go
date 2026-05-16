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

func TestCheckForUpdateWebhookRejectsSemVerOutsideRange(t *testing.T) {
	resource := deploymentWithContainer("gcr.io/v2-namespace/hello-world:1.25.3")

	_, update, err := checkForUpdate(mustSemVer(t, ">=1.25.0, <1.26.0"), nil, repo("gcr.io/v2-namespace/hello-world", "1.26.0"), "native", resource)
	if err != nil {
		t.Fatal(err)
	}
	if update {
		t.Fatal("expected semver range to reject webhook tag")
	}
}

func TestCheckForUpdateWebhookAllowsSemVerUpgrade(t *testing.T) {
	resource := deploymentWithContainer("gcr.io/v2-namespace/hello-world:1.25.3")

	plan, update, err := checkForUpdate(mustSemVer(t, ">=1.25.0, <1.26.0"), nil, repo("gcr.io/v2-namespace/hello-world", "1.25.4"), "native", resource)
	if err != nil {
		t.Fatal(err)
	}
	if !update {
		t.Fatal("expected update")
	}
	if plan.CurrentVersion != "1.25.3" || plan.NewVersion != "1.25.4" {
		t.Fatalf("plan version = %s -> %s", plan.CurrentVersion, plan.NewVersion)
	}
	if got := resource.Containers()[0].Image; got != "gcr.io/v2-namespace/hello-world:1.25.4" {
		t.Fatalf("container image = %q", got)
	}
}

func TestCheckForUpdateWebhookRejectsSemVerDowngrade(t *testing.T) {
	resource := deploymentWithContainer("gcr.io/v2-namespace/hello-world:1.25.3")

	_, update, err := checkForUpdate(mustSemVer(t, ">=1.25.0, <1.26.0"), nil, repo("gcr.io/v2-namespace/hello-world", "1.25.2"), "native", resource)
	if err != nil {
		t.Fatal(err)
	}
	if update {
		t.Fatal("expected downgrade to be rejected")
	}
}

func TestCheckForUpdateWebhookRejectsFilteredEventTag(t *testing.T) {
	filter := mustFilter(t, `^main-(?P<n>[0-9]+)$`, "$n")
	resource := deploymentWithContainer("gcr.io/v2-namespace/hello-world:main-100")

	_, update, err := checkForUpdate(mustNumerical(t, "desc"), filter, repo("gcr.io/v2-namespace/hello-world", "prod-200"), "native", resource)
	if err != nil {
		t.Fatal(err)
	}
	if update {
		t.Fatal("expected filter to reject event tag")
	}
}

func TestCheckForUpdateCurrentTagFilteredOutStillAllowsEventTag(t *testing.T) {
	filter := mustFilter(t, `^main-(?P<n>[0-9]+)$`, "$n")
	resource := deploymentWithContainer("gcr.io/v2-namespace/hello-world:legacy-100")

	_, update, err := checkForUpdate(mustNumerical(t, "desc"), filter, repo("gcr.io/v2-namespace/hello-world", "main-200"), "native", resource)
	if err != nil {
		t.Fatal(err)
	}
	if !update {
		t.Fatal("expected admitted event tag to update")
	}
}

func TestCheckForUpdateForceWebhookRespectsFilter(t *testing.T) {
	filter := mustFilter(t, `^release-[0-9]+$`, "")
	resource := deploymentWithContainer("gcr.io/v2-namespace/hello-world:release-1")

	_, update, err := checkForUpdate(policy.NewForce(), filter, repo("gcr.io/v2-namespace/hello-world", "dev-2"), "native", resource)
	if err != nil {
		t.Fatal(err)
	}
	if update {
		t.Fatal("expected filter to reject force webhook tag")
	}

	resource = deploymentWithContainer("gcr.io/v2-namespace/hello-world:release-1")
	_, update, err = checkForUpdate(policy.NewForce(), filter, repo("gcr.io/v2-namespace/hello-world", "release-2"), "native", resource)
	if err != nil {
		t.Fatal(err)
	}
	if !update {
		t.Fatal("expected filter-admitted force webhook tag to update")
	}
}

func TestCheckForUpdateNonPollTriggerNamesUseExternalAdmission(t *testing.T) {
	for _, triggerName := range []string{"pubsub", "", "unknown"} {
		t.Run(triggerName, func(t *testing.T) {
			resource := deploymentWithContainer("gcr.io/v2-namespace/hello-world:1.25.3")

			_, update, err := checkForUpdate(mustSemVer(t, ">=1.25.0, <1.26.0"), nil, repo("gcr.io/v2-namespace/hello-world", "1.26.0"), triggerName, resource)
			if err != nil {
				t.Fatal(err)
			}
			if update {
				t.Fatal("expected non-poll trigger to require policy admission")
			}
		})
	}
}

func TestCheckForUpdatePollTrustsWatcherSelectedTag(t *testing.T) {
	resource := deploymentWithContainer("gcr.io/v2-namespace/hello-world:1.25.3")

	_, update, err := checkForUpdate(mustSemVer(t, ">=1.25.0, <1.26.0"), nil, repo("gcr.io/v2-namespace/hello-world", "1.26.0"), types.TriggerTypePoll.String(), resource)
	if err != nil {
		t.Fatal(err)
	}
	if !update {
		t.Fatal("expected poll event to update without re-running policy")
	}
}

func TestCheckForUpdateSameTagNoop(t *testing.T) {
	resource := deploymentWithContainer("gcr.io/v2-namespace/hello-world:1.25.3")

	plan, update, err := checkForUpdate(nil, nil, repo("gcr.io/v2-namespace/hello-world", "1.25.3"), "native", resource)
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

func TestCheckForUpdateRejectsDifferentRepository(t *testing.T) {
	resource := deploymentWithContainer("gcr.io/v2-namespace/goodbye-world:1.0.0")

	plan, update, err := checkForUpdate(policy.NewForce(), nil, repo("gcr.io/v2-namespace/hello-world", "latest"), "native", resource)
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

	plan, update, err := checkForUpdate(policy.NewForce(), nil, repo("gcr.io/v2-namespace/hello-world", "new"), types.TriggerTypePoll.String(), resource)
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

func TestExternalEventRejectsNilPolicy(t *testing.T) {
	update, err := shouldUpdateFromEvent(nil, nil, "old", "new", "native")
	if err != nil {
		t.Fatal(err)
	}
	if update {
		t.Fatal("expected nil policy to reject external event")
	}
}

func repo(name, tag string) *types.Repository {
	return &types.Repository{Name: name, Tag: tag}
}

func mustSemVer(t *testing.T, constraint string) *policy.SemVer {
	t.Helper()
	plc, err := policy.NewSemVer(constraint)
	if err != nil {
		t.Fatal(err)
	}
	return plc
}

func mustNumerical(t *testing.T, order string) *policy.Numerical {
	t.Helper()
	plc, err := policy.NewNumerical(order)
	if err != nil {
		t.Fatal(err)
	}
	return plc
}

func mustFilter(t *testing.T, pattern, replace string) *policy.RegexFilter {
	t.Helper()
	filter, err := policy.NewRegexFilter(pattern, replace)
	if err != nil {
		t.Fatal(err)
	}
	return filter
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
