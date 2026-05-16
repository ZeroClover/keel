package poll

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/keel-hq/keel/extension/credentialshelper"
	"github.com/keel-hq/keel/internal/policy"
	"github.com/keel-hq/keel/provider"
	"github.com/keel-hq/keel/registry"
	"github.com/keel-hq/keel/types"
	"github.com/keel-hq/keel/util/image"
)

func mustParse(img string, schedule string) *types.TrackedImage {
	ref, err := image.Parse(img)
	if err != nil {
		panic(err)
	}
	return &types.TrackedImage{
		Image:        ref,
		PollSchedule: schedule,
		Trigger:      types.TriggerTypePoll,
		Policy:       mustSemVerPolicy(">=0.0.0-0"),
	}
}

func mustSemVerPolicy(r string) *policy.SemVer {
	plc, err := policy.NewSemVer(r)
	if err != nil {
		panic(err)
	}
	return plc
}

func mustRegexFilter(pattern, replace string) *policy.RegexFilter {
	filter, err := policy.NewRegexFilter(pattern, replace)
	if err != nil {
		panic(err)
	}
	return filter
}

type fakeRegistryClient struct {
	opts registry.Opts

	digestToReturn    string
	digestErrToReturn error
	digestByTag       map[string]string

	tagsToReturn []string

	createdByTag map[string]time.Time
	createdErr   map[string]error
	createdCalls []string
}

func (c *fakeRegistryClient) Get(opts registry.Opts) (*registry.Repository, error) {
	c.opts = opts
	return &registry.Repository{Name: opts.Name, Tags: c.tagsToReturn}, nil
}

func (c *fakeRegistryClient) Digest(opts registry.Opts) (string, error) {
	c.opts = opts
	if c.digestErrToReturn != nil {
		return "", c.digestErrToReturn
	}
	if digest, ok := c.digestByTag[opts.Tag]; ok {
		return digest, nil
	}
	if c.digestToReturn != "" {
		return c.digestToReturn, nil
	}
	return "sha256:" + opts.Tag, nil
}

func (c *fakeRegistryClient) GetCreatedTime(opts registry.Opts) (time.Time, error) {
	c.createdCalls = append(c.createdCalls, opts.Tag)
	if err, ok := c.createdErr[opts.Tag]; ok {
		return time.Time{}, err
	}
	return c.createdByTag[opts.Tag], nil
}

type fakeProvider struct {
	submitted []types.Event
	images    []*types.TrackedImage
}

func (p *fakeProvider) Submit(event types.Event) error {
	p.submitted = append(p.submitted, event)
	return nil
}

func (p *fakeProvider) GetName() string {
	return "fakeProvider"
}
func (p *fakeProvider) Stop() {}
func (p *fakeProvider) TrackedImages() ([]*types.TrackedImage, error) {
	return p.images, nil
}

func TestGetImageIdentifierDropsTag(t *testing.T) {
	ref, err := image.Parse("docker.io/library/nginx:1.25")
	if err != nil {
		t.Fatal(err)
	}
	got := getImageIdentifier(ref)
	if got != "index.docker.io/library/nginx" {
		t.Fatalf("identifier = %q", got)
	}
}

func TestWatchRepositoryTagsJob(t *testing.T) {
	reference, _ := image.Parse("foo/bar:1.1.0")
	fp := &fakeProvider{
		images: []*types.TrackedImage{{Image: reference, Policy: mustSemVerPolicy(">=0.0.0-0")}},
	}
	providers := provider.New([]provider.Provider{fp})
	frc := &fakeRegistryClient{tagsToReturn: []string{"1.1.2", "1.1.3", "0.9.1"}}
	details := &watchDetails{trackedImage: fp.images[0]}

	job := NewWatchRepositoryTagsJob(providers, frc, details, newCreatedTimeCache())
	job.Run()

	if len(fp.submitted) != 1 {
		t.Fatalf("submitted = %d, want 1", len(fp.submitted))
	}
	if fp.submitted[0].Repository.Name != "index.docker.io/foo/bar" {
		t.Fatalf("repository = %s", fp.submitted[0].Repository.Name)
	}
	if fp.submitted[0].Repository.Tag != "1.1.3" {
		t.Fatalf("tag = %s", fp.submitted[0].Repository.Tag)
	}
}

func TestWatchMultipleRepositories(t *testing.T) {
	imgA, _ := image.Parse("gcr.io/v2-namespace/hello-world:1.1.1")
	imgB, _ := image.Parse("gcr.io/v2-namespace/greetings-world:1.1.1")
	imgC, _ := image.Parse("gcr.io/v2-namespace/greetings-world:alpha")
	fp := &fakeProvider{images: []*types.TrackedImage{
		{Image: imgA, Trigger: types.TriggerTypePoll, PollSchedule: types.KeelPollDefaultSchedule, Policy: mustSemVerPolicy(">=0.0.0-0")},
		{Image: imgB, Trigger: types.TriggerTypePoll, PollSchedule: types.KeelPollDefaultSchedule, Policy: mustSemVerPolicy(">=0.0.0-0")},
		{Image: imgC, Trigger: types.TriggerTypePoll, PollSchedule: types.KeelPollDefaultSchedule, Policy: policy.NewForce()},
	}}
	providers := provider.New([]provider.Provider{fp})
	frc := &fakeRegistryClient{
		digestToReturn: "sha256:0604af35299dd37ff23937d115d103532948b568a9dd8197d14c256a8ab8b0bb",
		tagsToReturn:   []string{"5.0.0"},
	}
	watcher := NewRepositoryWatcher(providers, frc)

	err := watcher.Watch(fp.images...)
	if err != nil {
		t.Fatal(err)
	}
	if len(watcher.watched) != 2 {
		t.Fatalf("watched = %d, want 2", len(watcher.watched))
	}
	if _, ok := watcher.watched["gcr.io/v2-namespace/greetings-world"]; !ok {
		t.Fatal("greetings-world watcher not found")
	}
}

type fakeCredentialsHelper struct {
	getImageRequest *types.TrackedImage
	creds           *types.Credentials
	error           error
}

func (fch *fakeCredentialsHelper) GetCredentials(image *types.TrackedImage) (*types.Credentials, error) {
	fch.getImageRequest = image
	return fch.creds, fch.error
}

func (fch *fakeCredentialsHelper) IsEnabled() bool { return true }

func TestWatchRepositoryJobCheckCredentials(t *testing.T) {
	fakeHelper := &fakeCredentialsHelper{creds: &types.Credentials{Username: "user-xx", Password: "pass-xx"}}
	credentialshelper.RegisterCredentialsHelper("fake", fakeHelper)
	defer credentialshelper.UnregisterCredentialsHelper("fake")

	ref, _ := image.Parse("foo/bar:1.1")
	fp := &fakeProvider{images: []*types.TrackedImage{{Image: ref, Trigger: types.TriggerTypePoll, PollSchedule: "@every 10m", Policy: mustSemVerPolicy(">=0.0.0-0")}}}
	providers := provider.New([]provider.Provider{fp})
	frc := &fakeRegistryClient{digestToReturn: "sha256:0604af35299dd37ff23937d115d103532948b568a9dd8197d14c256a8ab8b0bb"}
	watcher := NewRepositoryWatcher(providers, frc)

	err := watcher.Watch(fp.images...)
	if err != nil {
		t.Fatal(err)
	}
	if frc.opts.Password != "pass-xx" || frc.opts.Username != "user-xx" {
		t.Fatalf("registry credentials = %s/%s", frc.opts.Username, frc.opts.Password)
	}
}

func TestWatchWithAuthenticationError(t *testing.T) {
	fakeHelper := &fakeCredentialsHelper{error: errors.New("no credentials found")}
	credentialshelper.RegisterCredentialsHelper("fake", fakeHelper)
	defer credentialshelper.UnregisterCredentialsHelper("fake")

	fp := &fakeProvider{}
	providers := provider.New([]provider.Provider{fp})
	frc := &fakeRegistryClient{digestErrToReturn: errors.New("authentication failed")}
	watcher := NewRepositoryWatcher(providers, frc)
	tracked := []*types.TrackedImage{mustParse("private.registry.com/v2-namespace/hello-world:1.1.1", "@every 10m")}

	err := watcher.Watch(tracked...)
	if err == nil {
		t.Fatal("expected authentication error")
	}
}

func TestUnwatchAfterNotTrackedAnymore(t *testing.T) {
	fp := &fakeProvider{}
	providers := provider.New([]provider.Provider{fp})
	frc := &fakeRegistryClient{
		digestToReturn: "sha256:0604af35299dd37ff23937d115d103532948b568a9dd8197d14c256a8ab8b0bb",
		tagsToReturn:   []string{"5.0.0"},
	}
	watcher := NewRepositoryWatcher(providers, frc)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher.Start(ctx)

	tracked := []*types.TrackedImage{
		mustParse("gcr.io/v2-namespace/hello-world:1.1.1", "@every 10m"),
		mustParse("gcr.io/v2-namespace/greetings-world:1.1.1", "@every 10m"),
		mustParse("gcr.io/v2-namespace/greetings-world:alpha", "@every 10m"),
	}
	watcher.Watch(tracked...)
	if len(watcher.watched) != 2 {
		t.Fatalf("watched = %d, want 2", len(watcher.watched))
	}

	trackedUpdated := []*types.TrackedImage{
		mustParse("gcr.io/v2-namespace/hello-world:1.1.1", "@every 10m"),
	}
	watcher.Watch(trackedUpdated...)
	if len(watcher.watched) != 1 {
		t.Fatalf("watched = %d, want 1", len(watcher.watched))
	}
}
