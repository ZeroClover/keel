package poll

import (
	"fmt"
	"sort"
	"time"

	"github.com/keel-hq/keel/extension/credentialshelper"
	"github.com/keel-hq/keel/provider"
	"github.com/keel-hq/keel/registry"
	"github.com/keel-hq/keel/types"
	"github.com/keel-hq/keel/util/image"

	"github.com/prometheus/client_golang/prometheus"

	log "github.com/sirupsen/logrus"
)

// WatchRepositoryTagsJob - watch all tags
type WatchRepositoryTagsJob struct {
	providers      provider.Providers
	registryClient registry.Client
	details        *watchDetails
	cache          *createdTimeCache

	// latests map[string]string // a map of prerelease tags and their corresponding latest versions
}

// NewWatchRepositoryTagsJob - new tags watcher job
func NewWatchRepositoryTagsJob(providers provider.Providers, registryClient registry.Client, details *watchDetails, cache *createdTimeCache) *WatchRepositoryTagsJob {
	return &WatchRepositoryTagsJob{
		providers:      providers,
		registryClient: registryClient,
		details:        details,
		cache:          cache,
		// latests:        details.trackedImage.SemverPreReleaseTags,
	}
}

// Run - main function to check schedule
func (j *WatchRepositoryTagsJob) Run() {
	j.details.mu.RLock()
	defer j.details.mu.RUnlock()

	reg := j.details.trackedImage.Image.Scheme() + "://" + j.details.trackedImage.Image.Registry()
	if j.details.latest == "" {
		j.details.latest = j.details.trackedImage.Image.Tag()
	}

	registryOpts := registry.Opts{
		Registry: reg,
		Name:     j.details.trackedImage.Image.ShortName(),
		Tag:      j.details.latest,
	}

	creds, err := credentialshelper.GetCredentials(j.details.trackedImage)
	if err == nil {
		registryOpts.Username = creds.Username
		registryOpts.Password = creds.Password
	}

	repository, err := j.registryClient.Get(registryOpts)

	if err != nil {
		log.WithFields(log.Fields{
			"error":        err,
			"registry_url": reg,
			"image":        j.details.trackedImage.Image.String(),
		}).Error("trigger.poll.WatchRepositoryTagsJob: failed to get repository")
		return
	}

	registriesScannedCounter.With(prometheus.Labels{"registry": j.details.trackedImage.Image.Registry(), "image": j.details.trackedImage.Image.Repository()}).Inc()

	log.WithFields(log.Fields{
		"current_tag":     j.details.trackedImage.Image.Tag(),
		"repository_tags": repository.Tags,
		"image_name":      j.details.trackedImage.Image.Remote(),
	}).Debug("trigger.poll.WatchRepositoryTagsJob: checking tags")

	err = j.processTags(repository.Tags)
	if err != nil {
		log.WithFields(log.Fields{
			"error":           err,
			"repository_tags": repository.Tags,
			"image":           j.details.trackedImage.Image.String(),
		}).Error("trigger.poll.WatchRepositoryTagsJob: failed to process tags")
		return
	}
}

func (j *WatchRepositoryTagsJob) computeEvents(tags []string) ([]types.Event, error) {
	trackedImages, err := j.providers.TrackedImages()
	if err != nil {
		return nil, err
	}

	events := []types.Event{}

	// This contains all tracked images that share the same imageIdentifier and thus, the same watcher
	allRelatedTrackedImages := getRelatedTrackedImages(j.details.trackedImage, trackedImages)

	for _, trackedImage := range allRelatedTrackedImages {
		if trackedImage.Policy == nil {
			continue
		}

		// The fact that they are related, does not mean they share the exact same Policy configuration, so wee need
		// to calculate the tags here for each image.
		filter := trackedImage.Filter
		candidates := tags
		if trackedImage.Policy.Type() == types.PolicyTypeForce {
			candidates = originalTagsForForce(tags, filter)
			if forcePolicySortsByCreated(trackedImage.Policy) {
				candidates = sortByCreatedTime(candidates, trackedImage.Image, j.registryClient, j.cache)
			}
		} else if filter != nil {
			filter.Apply(tags)
			candidates = filter.Items()
		}

		latestKey, err := trackedImage.Policy.Latest(candidates)
		if err != nil {
			log.WithFields(log.Fields{
				"error":  err,
				"image":  trackedImage.Image.String(),
				"policy": trackedImage.Policy.Name(),
			}).Debug("trigger.poll.WatchRepositoryTagsJob: failed to select latest tag")
			continue
		}

		latest := latestKey
		if trackedImage.Policy.Type() != types.PolicyTypeForce && filter != nil {
			latest = filter.GetOriginalTag(latestKey)
		}
		if latest == "" || trackedImage.Image.Tag() == latest || exists(latest, events) {
			continue
		}

		event := types.Event{
			Repository: types.Repository{
				Name: trackedImage.Image.Repository(),
				Tag:  latest,
			},
			TriggerName: types.TriggerTypePoll.String(),
		}
		events = append(events, event)
	}

	log.WithFields(log.Fields{
		"current_tag": j.details.trackedImage.Image.Tag(),
		"image_name":  j.details.trackedImage.Image.Remote(),
	}).Debug("trigger.poll.WatchRepositoryTagsJob: events: ", events)

	return events, nil
}

// forcePolicySortsByCreated reports whether the supplied force policy has
// opted into sorting candidate tags by their image config `created` time.
// Implementations expose this via a `SortByCreated() bool` method; policies
// that do not implement it default to off, preserving the fast path that
// avoids per-tag manifest API calls on large repositories.
func forcePolicySortsByCreated(plc types.Policy) bool {
	aware, ok := plc.(interface{ SortByCreated() bool })
	if !ok {
		return false
	}
	return aware.SortByCreated()
}

func originalTagsForForce(tags []string, filter types.Filter) []string {
	if filter == nil {
		return append([]string(nil), tags...)
	}

	filter.Apply(tags)
	items := filter.Items()
	originals := make([]string, 0, len(items))
	for _, key := range items {
		original := filter.GetOriginalTag(key)
		if original != "" {
			originals = append(originals, original)
		}
	}
	return originals
}

func sortByCreatedTime(tags []string, img *image.Reference, client registry.Client, cache *createdTimeCache) []string {
	type tagCreated struct {
		tag     string
		created time.Time
		ok      bool
	}

	registryURL := img.Scheme() + "://" + img.Registry()
	createdTags := make([]tagCreated, 0, len(tags))
	for _, tag := range tags {
		opts := registry.Opts{
			Registry: registryURL,
			Name:     img.ShortName(),
			Tag:      tag,
		}

		manifestDigest, err := client.Digest(opts)
		if err != nil || manifestDigest == "" {
			createdTags = append(createdTags, tagCreated{tag: tag})
			continue
		}

		cacheKey := fmt.Sprintf("%s/%s@%s", registryURL, img.ShortName(), manifestDigest)
		if created, ok := cache.Get(cacheKey); ok {
			createdTags = append(createdTags, tagCreated{tag: tag, created: created, ok: true})
			continue
		}

		created, err := client.GetCreatedTime(opts)
		if err != nil || created.IsZero() {
			createdTags = append(createdTags, tagCreated{tag: tag})
			continue
		}

		cache.Set(cacheKey, created)
		createdTags = append(createdTags, tagCreated{tag: tag, created: created, ok: true})
	}

	sort.Slice(createdTags, func(i, j int) bool {
		left, right := createdTags[i], createdTags[j]
		if left.ok != right.ok {
			return left.ok
		}
		if left.ok && !left.created.Equal(right.created) {
			return left.created.After(right.created)
		}
		return left.tag < right.tag
	})

	sorted := make([]string, 0, len(createdTags))
	for _, item := range createdTags {
		sorted = append(sorted, item.tag)
	}
	return sorted
}

func exists(tag string, events []types.Event) bool {
	for _, e := range events {
		if tag == e.Repository.Tag {
			return true
		}
	}
	return false
}

func getRelatedTrackedImages(ours *types.TrackedImage, all []*types.TrackedImage) []*types.TrackedImage {
	b := all[:0]
	for _, x := range all {
		if getImageIdentifier(x.Image) == getImageIdentifier(ours.Image) {
			b = append(b, x)
		}
	}
	return b
}

func (j *WatchRepositoryTagsJob) processTags(tags []string) error {

	events, err := j.computeEvents(tags)
	if err != nil {
		return err
	}
	for _, e := range events {
		err = j.providers.Submit(e)
		if err != nil {
			log.WithFields(log.Fields{
				"repository": j.details.trackedImage.Image.Repository(),
				"new_tag":    e.Repository.Tag,
				"error":      err,
			}).Error("trigger.poll.WatchRepositoryTagsJob: error while submitting an event")
		}
	}
	return nil
}
