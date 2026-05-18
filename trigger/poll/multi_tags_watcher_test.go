package poll

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/keel-hq/keel/extension/credentialshelper"
	"github.com/keel-hq/keel/internal/policy"
	"github.com/keel-hq/keel/provider"
	"github.com/keel-hq/keel/types"
	"github.com/keel-hq/keel/util/image"
)

func TestWatchMultipleTagsWithSemver(t *testing.T) {
	// fake provider listening for events
	imgA, _ := image.Parse("gcr.io/v2-namespace/hello-world:1.1.1")
	fp := &fakeProvider{
		images: []*types.TrackedImage{
			{
				Image:        imgA,
				Trigger:      types.TriggerTypePoll,
				Provider:     "fp",
				PollSchedule: types.KeelPollDefaultSchedule,
				Policy:       mustSemVerPolicy(">=0.0.0-0"),
			},
		},
	}
	providers := provider.New([]provider.Provider{fp})

	// returning some sha
	frc := &fakeRegistryClient{
		digestToReturn: "sha256:0604af35299dd37ff23937d115d103532948b568a9dd8197d14c256a8ab8b0bb",
		tagsToReturn:   []string{"5.0.0"},
	}

	watcher := NewRepositoryWatcher(providers, frc)

	tracked := []*types.TrackedImage{
		mustParse("gcr.io/v2-namespace/hello-world:1.1.1", "@every 10m"),
	}

	err := watcher.Watch(tracked...)
	if err != nil {
		t.Errorf("failed to watch: %s", err)
	}

	if len(watcher.watched) != 1 {
		t.Errorf("expected to find watching 1 entries, found: %d", len(watcher.watched))
	}
}

type runTestCase struct {
	currentTag  string
	expectedTag string
	bumpPolicy  policy.Policy
	filter      policy.Filter
}

// Helper function to factorize code
func testRunHelper(testCases []runTestCase, availableTags []string, t *testing.T) {
	var testImages []*types.TrackedImage
	for _, testCase := range testCases {
		reference, _ := image.Parse("foo/bar:" + testCase.currentTag)
		testImages = append(testImages, &types.TrackedImage{
			Image:  reference,
			Policy: testCase.bumpPolicy,
			Filter: testCase.filter,
		})
	}
	fp := &fakeProvider{
		images: testImages,
	}
	providers := provider.New([]provider.Provider{fp})

	frc := &fakeRegistryClient{
		tagsToReturn: availableTags,
	}

	details := &watchDetails{
		trackedImage: fp.images[0],
	}

	job := NewWatchRepositoryTagsJob(providers, frc, details, newCreatedTimeCache())

	job.Run()

	// Compute number of expected events (version bump expected)
	var nbEvents = 0
	for _, testCase := range testCases {
		if testCase.currentTag != testCase.expectedTag {
			nbEvents++
		}
	}
	// checking whether new job was submitted
	if len(fp.submitted) != nbEvents {
		tags := []string{}
		for _, s := range fp.submitted {
			tags = append(tags, s.Repository.Tag)
		}
		t.Errorf("expected "+strconv.Itoa(nbEvents)+" events, got: %d [%s]", len(fp.submitted), strings.Join(tags, ", "))
	} else {
		for i, testCase := range testCases {
			submitted := fp.submitted[i]

			if submitted.Repository.Name != "index.docker.io/foo/bar" {
				t.Errorf("unexpected event repository name: %s", submitted.Repository.Name)
			}

			if submitted.Repository.Tag != testCase.expectedTag {
				t.Errorf("expected event repository tag "+testCase.expectedTag+", but got: %s", submitted.Repository.Tag)
			}
		}
	}
}

func TestWatchAllTagsJobWith2pointSemver(t *testing.T) {
	availableTags := []string{"1.3", "2.5", "2.7", "3.8"}
	testRunHelper([]runTestCase{{currentTag: "1.3", expectedTag: "3.8", bumpPolicy: mustSemVerPolicy(">=0.0.0-0")}}, availableTags, t)
	testRunHelper([]runTestCase{{currentTag: "2.5", expectedTag: "3.8", bumpPolicy: mustSemVerPolicy(">=0.0.0-0")}}, availableTags, t)
	testRunHelper([]runTestCase{{currentTag: "2.5", expectedTag: "2.7", bumpPolicy: mustSemVerPolicy(">=2.0.0-0, <3.0.0-0")}}, availableTags, t)
}

func TestWatchAllTagsJobWithSemver(t *testing.T) {
	availableTags := []string{"1.3.0-dev", "1.5.0", "1.8.0-alpha"}
	testCases := []runTestCase{{currentTag: "1.1.0", expectedTag: "1.5.0", bumpPolicy: mustSemVerPolicy(">=0.0.0")}}
	testRunHelper(testCases, availableTags, t)
}

func TestWatchAllTagsPrerelease(t *testing.T) {
	availableTags := []string{"1.3.0-dev", "1.5.0", "1.8.0-alpha"}
	testCases := []runTestCase{{
		currentTag:  "1.2.0-dev",
		expectedTag: "1.3.0-dev",
		bumpPolicy:  mustSemVerPolicy(">=0.0.0-0"),
		filter:      mustRegexFilter(`^.*-dev$`, ""),
	}}
	testRunHelper(testCases, availableTags, t)
}

// Full Semver, including pre-releases
func TestWatchAllTagsFullSemver(t *testing.T) {
	availableTags := []string{"1.3.0-dev", "1.5.0", "1.8.0-alpha"}
	testCases := []runTestCase{{currentTag: "1.2.0-dev", expectedTag: "1.8.0-alpha", bumpPolicy: mustSemVerPolicy(">=0.0.0-0")}}
	testRunHelper(testCases, availableTags, t)

	// Test simulating linuxserver tagging strategy
	availableTags = []string{"v0.1.2-ls1", "v0.1.2-ls2", "v0.1.3-ls1", "v0.1.3-ls2", "v0.2.0-ls2", "v0.2.0-ls3"}
	testCases = []runTestCase{{currentTag: "v0.1.0-ls1", expectedTag: "v0.2.0-ls3", bumpPolicy: mustSemVerPolicy(">=0.0.0-0")}}
	testRunHelper(testCases, availableTags, t)

}

func TestWatchAllTagsHiddenMinorWith2PointVersions(t *testing.T) {
	availableTags := []string{"1.3", "1.5", "2.0", "1.2.1"}
	testRunHelper([]runTestCase{{currentTag: "1.2", expectedTag: "1.2.1", bumpPolicy: mustSemVerPolicy("~1.2")}}, availableTags, t)
	testRunHelper([]runTestCase{{currentTag: "1.2", expectedTag: "1.5", bumpPolicy: mustSemVerPolicy(">=1.0.0-0, <2.0.0-0")}}, availableTags, t)
	testRunHelper([]runTestCase{{currentTag: "1.2", expectedTag: "2.0", bumpPolicy: mustSemVerPolicy(">=0.0.0-0")}}, availableTags, t)
}

// Bug #490: new major version "hiding" minor one
func TestWatchAllTagsHiddenMinor(t *testing.T) {
	availableTags := []string{"1.3.0", "1.5.0", "2.0.0", "1.2.1"}
	testRunHelper([]runTestCase{{currentTag: "1.2.0", expectedTag: "1.2.1", bumpPolicy: mustSemVerPolicy("~1.2")}}, availableTags, t)
	testRunHelper([]runTestCase{{currentTag: "1.2.0", expectedTag: "1.5.0", bumpPolicy: mustSemVerPolicy(">=1.0.0-0, <2.0.0-0")}}, availableTags, t)
	testRunHelper([]runTestCase{{currentTag: "1.2.0", expectedTag: "2.0.0", bumpPolicy: mustSemVerPolicy(">=0.0.0-0")}}, availableTags, t)
}

func TestWatchAllTagsMixed(t *testing.T) {
	availableTags := []string{"1.3.0-dev", "1.5.0", "1.8.0-alpha"}
	testCases := []runTestCase{
		{currentTag: "1.0.0", expectedTag: "1.5.0", bumpPolicy: mustSemVerPolicy(">=0.0.0")},
		{currentTag: "1.2.0-dev", expectedTag: "1.3.0-dev", bumpPolicy: mustSemVerPolicy(">=0.0.0-0"), filter: mustRegexFilter(`^.*-dev$`, "")}}
	testRunHelper(testCases, availableTags, t)
}

func TestWatchGlobTagsMixed(t *testing.T) {
	availableTags := []string{"1.3.0-dev", "build-1694132169", "build-1696801785", "build-1695801785"}
	plc, _ := policy.NewAlphabetical("desc")
	testCases := []runTestCase{
		{currentTag: "1.0.0", expectedTag: "build-1696801785", bumpPolicy: plc, filter: mustRegexFilter(`^build-.*$`, "")},
	}
	testRunHelper(testCases, availableTags, t)
}

func TestWatchRegexpTagsCompareMixed(t *testing.T) {
	availableTags := []string{"1.3.0-dev", "build-2a3560ef-1694132169", "build-1a3560ef-1696801785", "build-3a3560ef-1695801785"}
	plc, _ := policy.NewNumerical("desc")
	testCases := []runTestCase{
		{currentTag: "1.0.0", expectedTag: "build-1a3560ef-1696801785", bumpPolicy: plc, filter: mustRegexFilter(`^build-.*-(?P<compare>[0-9]+)$`, "$compare")},
	}
	testRunHelper(testCases, availableTags, t)
}

func TestWatchRegexpTagsMixed(t *testing.T) {
	availableTags := []string{"1.3.0-dev", "build-2a3560ef-1694132169", "build-1a3560ef-1696801785", "build-3a3560ef-1695801785"}
	plc, _ := policy.NewAlphabetical("desc")
	testCases := []runTestCase{
		{currentTag: "1.0.0", expectedTag: "build-3a3560ef-1695801785", bumpPolicy: plc, filter: mustRegexFilter(`^build-.*$`, "")},
	}
	testRunHelper(testCases, availableTags, t)
}

func TestWatchAllTagsMixedPolicyAll(t *testing.T) {
	availableTags := []string{"1.3.0-dev", "1.5.0", "1.8.0-alpha"}
	testCases := []runTestCase{
		{currentTag: "1.0.0", expectedTag: "1.5.0", bumpPolicy: mustSemVerPolicy(">=0.0.0")},
		{currentTag: "1.6.0-alpha", expectedTag: "1.8.0-alpha", bumpPolicy: mustSemVerPolicy(">=0.0.0-0")}}
	testRunHelper(testCases, availableTags, t)
}

func TestForcePolicySortsByCreatedTime(t *testing.T) {
	fp, job, _ := forceJob("tag-A", []string{"tag-A", "tag-B", "tag-C"}, nil, &fakeRegistryClient{
		digestByTag: map[string]string{
			"tag-A": "sha256:a",
			"tag-B": "sha256:b",
			"tag-C": "sha256:c",
		},
		createdByTag: map[string]time.Time{
			"tag-A": time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			"tag-B": time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
			"tag-C": time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		},
	}, newCreatedTimeCache())

	job.Run()

	if len(fp.submitted) != 1 {
		t.Fatalf("submitted = %d, want 1", len(fp.submitted))
	}
	if fp.submitted[0].Repository.Tag != "tag-B" {
		t.Fatalf("tag = %s, want tag-B", fp.submitted[0].Repository.Tag)
	}
}

func TestForcePolicyMissingCreatedTimeGoesLastAndIsNotCached(t *testing.T) {
	cache := newCreatedTimeCache()
	frc := &fakeRegistryClient{
		digestByTag: map[string]string{
			"a": "sha256:a",
			"b": "sha256:b",
			"c": "sha256:c",
		},
		createdByTag: map[string]time.Time{
			"a": time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			"b": time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		},
		createdErr: map[string]error{"c": errors.New("missing created")},
	}
	fp, job, _ := forceJob("a", []string{"a", "b", "c"}, nil, frc, cache)

	events, err := job.computeEvents([]string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Repository.Tag != "b" {
		t.Fatalf("events = %#v, want tag b", events)
	}
	events, err = job.computeEvents([]string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Repository.Tag != "b" {
		t.Fatalf("events = %#v, want tag b", events)
	}

	if countString(frc.createdCalls, "a") != 1 || countString(frc.createdCalls, "b") != 1 {
		t.Fatalf("successful created calls were not cached: %#v", frc.createdCalls)
	}
	if countString(frc.createdCalls, "c") != 2 {
		t.Fatalf("failed created call should not be cached: %#v", frc.createdCalls)
	}
	if _, ok := cache.Get("https://index.docker.io/foo/bar@sha256:c"); ok {
		t.Fatal("failed created time was cached")
	}
	if len(fp.submitted) != 0 {
		t.Fatalf("computeEvents should not submit events directly")
	}
}

func TestForcePolicyWithFilterUsesOriginalTagsForCreatedTime(t *testing.T) {
	filter := mustRegexFilter(`^release-(?P<n>[0-9]+)$`, "$n")
	frc := &fakeRegistryClient{
		digestByTag: map[string]string{
			"release-1": "sha256:release-1",
			"release-2": "sha256:release-2",
		},
		createdByTag: map[string]time.Time{
			"release-1": time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			"release-2": time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	fp, job, _ := forceJob("release-1", []string{"release-1", "release-2", "dev-3"}, filter, frc, newCreatedTimeCache())

	job.Run()

	if len(fp.submitted) != 1 {
		t.Fatalf("submitted = %d, want 1", len(fp.submitted))
	}
	if fp.submitted[0].Repository.Tag != "release-2" {
		t.Fatalf("tag = %s, want release-2", fp.submitted[0].Repository.Tag)
	}
	if strings.Join(frc.createdCalls, ",") != "release-1,release-2" {
		t.Fatalf("created calls = %#v, want original release tags", frc.createdCalls)
	}
}

func TestForcePolicyDefaultSkipsCreatedTime(t *testing.T) {
	frc := &fakeRegistryClient{
		digestByTag: map[string]string{
			"tag-A": "sha256:a",
			"tag-B": "sha256:b",
			"tag-C": "sha256:c",
		},
		createdByTag: map[string]time.Time{
			"tag-A": time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			"tag-B": time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
			"tag-C": time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	fp, job, _ := tagsJobWithCache("current", []string{"tag-A", "tag-B", "tag-C"}, policy.NewForce(), nil, frc, newCreatedTimeCache())

	job.Run()

	if len(fp.submitted) != 1 {
		t.Fatalf("submitted = %d, want 1", len(fp.submitted))
	}
	if fp.submitted[0].Repository.Tag != "tag-A" {
		t.Fatalf("tag = %s, want tag-A (first in registry order, no created sort)", fp.submitted[0].Repository.Tag)
	}
	if len(frc.createdCalls) != 0 {
		t.Fatalf("created time calls = %#v, want none on default force policy", frc.createdCalls)
	}
}

func TestSemverPolicyBypassesCreatedTime(t *testing.T) {
	fp, job := tagsJob("1.0.0", []string{"1.0.0", "1.1.0"}, mustSemVerPolicy(">=0.0.0"), nil, &fakeRegistryClient{
		digestByTag: map[string]string{
			"1.0.0": "sha256:1",
			"1.1.0": "sha256:2",
		},
		createdByTag: map[string]time.Time{
			"1.1.0": time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		},
	})

	job.Run()

	if len(fp.submitted) != 1 {
		t.Fatalf("submitted = %d, want 1", len(fp.submitted))
	}
	if fp.submitted[0].Repository.Tag != "1.1.0" {
		t.Fatalf("tag = %s, want 1.1.0", fp.submitted[0].Repository.Tag)
	}
	frc := job.registryClient.(*fakeRegistryClient)
	if len(frc.createdCalls) != 0 {
		t.Fatalf("created time calls = %#v, want none", frc.createdCalls)
	}
}

func forceJob(currentTag string, availableTags []string, filter policy.Filter, frc *fakeRegistryClient, cache *createdTimeCache) (*fakeProvider, *WatchRepositoryTagsJob, *types.TrackedImage) {
	return tagsJobWithCache(currentTag, availableTags, mustForcePolicy("created"), filter, frc, cache)
}

func mustForcePolicy(option string) *policy.Force {
	plc, err := policy.NewForceWithOption(option)
	if err != nil {
		panic(err)
	}
	return plc
}

func tagsJob(currentTag string, availableTags []string, plc policy.Policy, filter policy.Filter, frc *fakeRegistryClient) (*fakeProvider, *WatchRepositoryTagsJob) {
	fp, job, _ := tagsJobWithCache(currentTag, availableTags, plc, filter, frc, newCreatedTimeCache())
	return fp, job
}

func tagsJobWithCache(currentTag string, availableTags []string, plc policy.Policy, filter policy.Filter, frc *fakeRegistryClient, cache *createdTimeCache) (*fakeProvider, *WatchRepositoryTagsJob, *types.TrackedImage) {
	reference, _ := image.Parse("foo/bar:" + currentTag)
	tracked := &types.TrackedImage{
		Image:  reference,
		Policy: plc,
		Filter: filter,
	}
	fp := &fakeProvider{images: []*types.TrackedImage{tracked}}
	if frc.tagsToReturn == nil {
		frc.tagsToReturn = availableTags
	}
	job := NewWatchRepositoryTagsJob(provider.New([]provider.Provider{fp}), frc, &watchDetails{trackedImage: tracked}, cache)
	return fp, job, tracked
}

func countString(items []string, want string) int {
	count := 0
	for _, item := range items {
		if item == want {
			count++
		}
	}
	return count
}

type testingCredsHelper struct {
	err         error              // err to return
	credentials *types.Credentials // creds to return
}

func (h *testingCredsHelper) IsEnabled() bool {
	return true
}

func (h *testingCredsHelper) GetCredentials(image *types.TrackedImage) (*types.Credentials, error) {
	return h.credentials, h.err
}

func TestWatchMultipleTagsWithCredentialsHelper(t *testing.T) {
	// fake provider listening for events
	imgA, _ := image.Parse("gcr.io/v2-namespace/hello-world:1.1.1")
	fp := &fakeProvider{
		images: []*types.TrackedImage{
			{
				Image:        imgA,
				Trigger:      types.TriggerTypePoll,
				Provider:     "fp",
				PollSchedule: types.KeelPollDefaultSchedule,
				Policy:       mustSemVerPolicy(">=0.0.0-0"),
			},
		},
	}
	t.Run("TestError", func(t *testing.T) {
		mockHelper := &testingCredsHelper{
			err: errors.New("doesn't work"),
		}
		credentialshelper.RegisterCredentialsHelper("mock", mockHelper)
		defer credentialshelper.UnregisterCredentialsHelper("mock")

		providers := provider.New([]provider.Provider{fp})

		// returning some sha
		frc := &fakeRegistryClient{
			digestToReturn: "sha256:0604af35299dd37ff23937d115d103532948b568a9dd8197d14c256a8ab8b0bb",
			tagsToReturn:   []string{"5.0.0"},
		}

		watcher := NewRepositoryWatcher(providers, frc)

		tracked := []*types.TrackedImage{
			mustParse("gcr.io/v2-namespace/hello-world:1.1.1", "@every 10m"),
		}

		err := watcher.Watch(tracked...)
		if err != nil {
			t.Errorf("failed to watch: %s", err)
		}

		if len(watcher.watched) != 1 {
			t.Errorf("expected to find watching 1 entries, found: %d", len(watcher.watched))
		}
		assert.Equal(t, "", frc.opts.Username)
		assert.Equal(t, "", frc.opts.Password)
	})

	t.Run("TestOK", func(t *testing.T) {
		mockHelper := &testingCredsHelper{
			err: nil,
			credentials: &types.Credentials{
				Username: "user",
				Password: "pass",
			},
		}
		credentialshelper.RegisterCredentialsHelper("mock", mockHelper)
		defer credentialshelper.UnregisterCredentialsHelper("mock")

		providers := provider.New([]provider.Provider{fp})

		// returning some sha
		frc := &fakeRegistryClient{
			digestToReturn: "sha256:0604af35299dd37ff23937d115d103532948b568a9dd8197d14c256a8ab8b0bb",
			tagsToReturn:   []string{"5.0.0"},
		}

		watcher := NewRepositoryWatcher(providers, frc)

		tracked := []*types.TrackedImage{
			mustParse("gcr.io/v2-namespace/hello-world:1.1.1", "@every 10m"),
		}

		err := watcher.Watch(tracked...)
		if err != nil {
			t.Errorf("failed to watch: %s", err)
		}

		if len(watcher.watched) != 1 {
			t.Errorf("expected to find watching 1 entries, found: %d", len(watcher.watched))
		}
		assert.Equal(t, "user", frc.opts.Username)
		assert.Equal(t, "pass", frc.opts.Password)
	})

}
