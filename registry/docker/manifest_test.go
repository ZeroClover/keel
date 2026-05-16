package docker

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestManifestDigest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Fatalf("method = %s, want HEAD", r.Method)
		}
		w.Header().Set("Docker-Content-Digest", "sha256:7aa5175f39a7e8a4172972524302c9a8196f681e40d6ee5d2f6bf0ab7d600fee")
	}))
	defer ts.Close()

	reg := New(ts.URL, "", "")

	digest, err := reg.ManifestDigest("teapot/external-dns", "v0.4.8")
	if err != nil {
		t.Fatal(err)
	}
	if digest.String() != "sha256:7aa5175f39a7e8a4172972524302c9a8196f681e40d6ee5d2f6bf0ab7d600fee" {
		t.Fatalf("digest = %s", digest.String())
	}
}

func TestGetCreatedTime(t *testing.T) {
	created := "2024-05-01T10:00:00Z"
	indexCreated := "2025-01-01T00:00:00Z"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/app/manifests/v1":
			w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			fmt.Fprint(w, `{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json","config":{"mediaType":"application/vnd.docker.container.image.v1+json","digest":"sha256:abc","size":123}}`)
		case "/v2/app/blobs/sha256:abc":
			fmt.Fprintf(w, `{"created":%q}`, created)
		case "/v2/app/manifests/latest":
			w.Header().Set("Content-Type", "application/vnd.oci.image.index.v1+json")
			fmt.Fprint(w, `{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:child","size":123}]}`)
		case "/v2/app/manifests/sha256:child":
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			fmt.Fprint(w, `{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"mediaType":"application/vnd.oci.image.config.v1+json","digest":"sha256:def","size":123}}`)
		case "/v2/app/blobs/sha256:def":
			fmt.Fprintf(w, `{"created":%q}`, indexCreated)
		case "/v2/app/manifests/missing-created":
			w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			fmt.Fprint(w, `{"schemaVersion":2,"config":{"digest":"sha256:missing"}}`)
		case "/v2/app/blobs/sha256:missing":
			fmt.Fprint(w, `{}`)
		case "/v2/app/manifests/fail":
			http.Error(w, "nope", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	reg := New(ts.URL, "", "")

	got, err := reg.GetCreatedTime("app", "v1")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := time.Parse(time.RFC3339, created)
	if !got.Equal(want) {
		t.Fatalf("created = %s, want %s", got, want)
	}

	got, err = reg.GetCreatedTime("app", "latest")
	if err != nil {
		t.Fatal(err)
	}
	want, _ = time.Parse(time.RFC3339, indexCreated)
	if !got.Equal(want) {
		t.Fatalf("index created = %s, want %s", got, want)
	}

	got, err = reg.GetCreatedTime("app", "missing-created")
	if err == nil {
		t.Fatal("expected missing created error")
	}
	if !got.IsZero() {
		t.Fatalf("created = %s, want zero", got)
	}

	got, err = reg.GetCreatedTime("app", "fail")
	if err == nil {
		t.Fatal("expected http error")
	}
	if !got.IsZero() {
		t.Fatalf("created = %s, want zero", got)
	}
}
