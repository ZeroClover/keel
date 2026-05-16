package kubernetes

import (
	"testing"

	"github.com/keel-hq/keel/extension/notification"
	"github.com/keel-hq/keel/internal/k8s"
	"github.com/keel-hq/keel/types"

	apps_v1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	core_v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type fakeProvider struct {
	submitted []types.Event
	images    []*types.TrackedImage
}

func (p *fakeProvider) Submit(event types.Event) error {
	p.submitted = append(p.submitted, event)
	return nil
}

func (p *fakeProvider) TrackedImages() ([]*types.TrackedImage, error) {
	return p.images, nil
}
func (p *fakeProvider) List() []string {
	return []string{"fakeprovider"}
}
func (p *fakeProvider) Stop() {}
func (p *fakeProvider) GetName() string {
	return "fp"
}

type fakeImplementer struct {
	namespaces *v1.NamespaceList
	updated    *k8s.GenericResource
}

func (i *fakeImplementer) Namespaces() (*v1.NamespaceList, error) {
	return i.namespaces, nil
}

func (i *fakeImplementer) Deployment(namespace, name string) (*apps_v1.Deployment, error) {
	return nil, nil
}

func (i *fakeImplementer) Deployments(namespace string) (*apps_v1.DeploymentList, error) {
	return nil, nil
}

func (i *fakeImplementer) Update(obj *k8s.GenericResource) error {
	i.updated = obj
	return nil
}

func (i *fakeImplementer) Secret(namespace, name string) (*v1.Secret, error) {
	return nil, nil
}

func (i *fakeImplementer) Pods(namespace, labelSelector string) (*v1.PodList, error) {
	return nil, nil
}

func (i *fakeImplementer) DeletePod(namespace, name string, opts *meta_v1.DeleteOptions) error {
	return nil
}

func (i *fakeImplementer) ConfigMaps(namespace string) core_v1.ConfigMapInterface {
	return nil
}

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

func TestGetNamespaces(t *testing.T) {
	fi := &fakeImplementer{
		namespaces: &v1.NamespaceList{
			Items: []v1.Namespace{{ObjectMeta: meta_v1.ObjectMeta{Name: "xxxx"}}},
		},
	}

	grc := &k8s.GenericResourceCache{}
	provider, err := NewProvider(fi, &fakeSender{}, grc)
	if err != nil {
		t.Fatalf("failed to get provider: %s", err)
	}

	namespaces, err := provider.namespaces()
	if err != nil {
		t.Errorf("failed to get namespaces: %s", err)
	}
	if namespaces.Items[0].Name != "xxxx" {
		t.Errorf("expected xxxx but got %s", namespaces.Items[0].Name)
	}
}

func TestGetImageName(t *testing.T) {
	name := versionreg.ReplaceAllString("gcr.io/v2-namespace/hello-world:1.1", "")
	if name != "gcr.io/v2-namespace/hello-world" {
		t.Errorf("expected image name but got %q", name)
	}
}

func MustParseGR(obj interface{}) *k8s.GenericResource {
	gr, err := k8s.NewGenericResource(obj)
	if err != nil {
		panic(err)
	}
	return gr
}

func MustParseGRS(objs []*apps_v1.Deployment) []*k8s.GenericResource {
	grs := make([]*k8s.GenericResource, len(objs))
	for idx, obj := range objs {
		grs[idx] = MustParseGR(obj)
	}
	return grs
}

func TestCreateUpdatePlansWithSemVerPolicy(t *testing.T) {
	grc := &k8s.GenericResourceCache{}
	grc.Add(MustParseGR(deployment("dep-1", map[string]string{types.KeelPolicyLabel: "semver:>=0.0.0"}, nil, "gcr.io/v2-namespace/hello-world:1.1.1")))
	grc.Add(MustParseGR(deployment("dep-2", map[string]string{}, nil, "gcr.io/v2-namespace/hello-world:1.1.1")))

	provider, err := NewProvider(&fakeImplementer{}, &fakeSender{}, grc)
	if err != nil {
		t.Fatal(err)
	}

	plans, err := provider.createUpdatePlans(&types.Event{
		Repository:  types.Repository{Name: "gcr.io/v2-namespace/hello-world", Tag: "1.1.2"},
		TriggerName: types.TriggerTypePoll.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 {
		t.Fatalf("len(plans) = %d, want 1", len(plans))
	}
	if plans[0].Resource.Identifier != "deployment/default/dep-1" {
		t.Fatalf("plan resource = %s", plans[0].Resource.Identifier)
	}
}

func TestCreateUpdatePlansSkipsLegacyPolicy(t *testing.T) {
	grc := &k8s.GenericResourceCache{}
	grc.Add(MustParseGR(deployment("dep-1", map[string]string{types.KeelPolicyLabel: "all"}, nil, "gcr.io/v2-namespace/hello-world:1.1.1")))

	provider, err := NewProvider(&fakeImplementer{}, &fakeSender{}, grc)
	if err != nil {
		t.Fatal(err)
	}

	plans, err := provider.createUpdatePlans(&types.Event{
		Repository:  types.Repository{Name: "gcr.io/v2-namespace/hello-world", Tag: "1.1.2"},
		TriggerName: types.TriggerTypePoll.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 0 {
		t.Fatalf("len(plans) = %d, want 0", len(plans))
	}
}

func TestTrackedImagesCarriesFilter(t *testing.T) {
	grc := &k8s.GenericResourceCache{}
	grc.Add(MustParseGR(deployment("dep-1", nil, map[string]string{
		types.KeelPolicyLabel:          "numerical:desc",
		types.KeelFilterTagsAnnotation: "^main-[a-f0-9]+-(?P<ts>[0-9]+)$",
		types.KeelExtractAnnotation:    "$ts",
	}, "gcr.io/v2-namespace/hello-world:main-abc-100")))

	provider, err := NewProvider(&fakeImplementer{}, &fakeSender{}, grc)
	if err != nil {
		t.Fatal(err)
	}

	tracked, err := provider.TrackedImages()
	if err != nil {
		t.Fatal(err)
	}
	if len(tracked) != 1 {
		t.Fatalf("len(tracked) = %d, want 1", len(tracked))
	}
	if tracked[0].Policy == nil || tracked[0].Policy.Type() != types.PolicyTypeNumerical {
		t.Fatalf("policy = %#v", tracked[0].Policy)
	}
	if tracked[0].Filter == nil {
		t.Fatal("expected filter")
	}
}

func TestProcessEvent(t *testing.T) {
	grc := &k8s.GenericResourceCache{}
	grc.Add(MustParseGR(deployment("deployment-1", map[string]string{types.KeelPolicyLabel: "semver:>=0.0.0"}, nil, "gcr.io/v2-namespace/hello-world:10.0.0")))

	impl := &fakeImplementer{}
	sender := &fakeSender{}
	provider, err := NewProvider(impl, sender, grc)
	if err != nil {
		t.Fatal(err)
	}

	repo := types.Repository{Name: "gcr.io/v2-namespace/hello-world", Tag: "11.0.0"}
	_, err = provider.processEvent(&types.Event{Repository: repo})
	if err != nil {
		t.Fatal(err)
	}

	if impl.updated == nil {
		t.Fatal("resource was not updated")
	}
	if got := impl.updated.Containers()[0].Image; got != repo.Name+":"+repo.Tag {
		t.Fatalf("updated image = %s", got)
	}
	if sender.sentEvent.Message == "" {
		t.Fatal("expected success notification")
	}
}

func deployment(name string, labels map[string]string, annotations map[string]string, image string) *apps_v1.Deployment {
	if labels == nil {
		labels = map[string]string{}
	}
	if annotations == nil {
		annotations = map[string]string{}
	}
	return &apps_v1.Deployment{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:        name,
			Namespace:   "default",
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: apps_v1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: meta_v1.ObjectMeta{Annotations: map[string]string{}},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{Image: image}},
				},
			},
		},
	}
}
