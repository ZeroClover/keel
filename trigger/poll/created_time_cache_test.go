package poll

import (
	"testing"
	"time"
)

func TestCreatedTimeCacheGetSet(t *testing.T) {
	cache := newCreatedTimeCache()
	key := "docker.io/library/nginx@sha256:abc"
	created := time.Date(2024, 5, 1, 10, 0, 0, 0, time.UTC)

	if _, ok := cache.Get(key); ok {
		t.Fatal("expected cache miss")
	}

	cache.Set(key, created)

	got, ok := cache.Get(key)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if !got.Equal(created) {
		t.Fatalf("created = %s, want %s", got, created)
	}
}

func TestCreatedTimeCacheDoesNotStoreZeroTime(t *testing.T) {
	cache := newCreatedTimeCache()
	key := "docker.io/library/nginx@sha256:bad"

	cache.Set(key, time.Time{})

	if _, ok := cache.Get(key); ok {
		t.Fatal("expected zero time to be skipped")
	}
}
