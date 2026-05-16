package docker

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	manifestv2 "github.com/distribution/distribution/v3/manifest/schema2"
	"github.com/opencontainers/go-digest"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
)

const mediaTypeDockerManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"

// ManifestDigest - get manifest digest
func (r *Registry) ManifestDigest(repository, reference string) (digest.Digest, error) {
	url := r.url("/v2/%s/manifests/%s", repository, reference)
	r.Logf("registry.manifest.head url=%s repository=%s reference=%s", url, repository, reference)

	// Try HEAD request first because it's free
	resp, err := r.request("HEAD", url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if hdr := resp.Header.Get("Docker-Content-Digest"); hdr != "" {
		return digest.Parse(hdr)
	}

	// HEAD request didn't return a digest, attempt to fetch digest from body
	r.Logf("registry.manifest.get url=%s repository=%s reference=%s", url, repository, reference)
	resp, err = r.request("GET", url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Try to get digest from body instead, should be equal to what would be presented
	// in Docker-Content-Digest
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return digest.FromBytes(body), nil
}

// GetCreatedTime returns the image config created timestamp for a tag or manifest digest.
func (r *Registry) GetCreatedTime(repository, reference string) (time.Time, error) {
	configDigest, err := r.getConfigDigest(repository, reference)
	if err != nil {
		return time.Time{}, err
	}

	url := r.url("/v2/%s/blobs/%s", repository, configDigest)
	r.Logf("registry.config.get url=%s repository=%s reference=%s digest=%s", url, repository, reference, configDigest)
	resp, err := r.request("GET", url)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()

	var config struct {
		Created string `json:"created"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return time.Time{}, fmt.Errorf("failed to parse image config: %w", err)
	}
	if config.Created == "" {
		return time.Time{}, fmt.Errorf("image config missing created field")
	}
	created, err := time.Parse(time.RFC3339, config.Created)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse image config created field %q: %w", config.Created, err)
	}
	return created, nil
}

func (r *Registry) getConfigDigest(repository, reference string) (string, error) {
	url := r.url("/v2/%s/manifests/%s", repository, reference)
	r.Logf("registry.manifest.get url=%s repository=%s reference=%s", url, repository, reference)
	resp, err := r.request("GET", url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var manifest struct {
		MediaType string `json:"mediaType"`
		Config    struct {
			Digest string `json:"digest"`
		} `json:"config"`
		Manifests []struct {
			Digest string `json:"digest"`
		} `json:"manifests"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return "", fmt.Errorf("failed to parse manifest %s: %w", reference, err)
	}

	mediaType := manifest.MediaType
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		parsed, _, err := mime.ParseMediaType(contentType)
		if err == nil && parsed != "" {
			mediaType = parsed
		}
	}

	switch mediaType {
	case oci.MediaTypeImageIndex, mediaTypeDockerManifestList:
		if len(manifest.Manifests) == 0 || manifest.Manifests[0].Digest == "" {
			return "", fmt.Errorf("manifest index %s has no child manifests", reference)
		}
		return r.getConfigDigest(repository, manifest.Manifests[0].Digest)
	case manifestv2.MediaTypeManifest, oci.MediaTypeImageManifest, "":
		if manifest.Config.Digest == "" {
			return "", fmt.Errorf("manifest %s missing config digest", reference)
		}
		return manifest.Config.Digest, nil
	default:
		if manifest.Config.Digest != "" {
			return manifest.Config.Digest, nil
		}
		return "", fmt.Errorf("unsupported manifest media type %q", mediaType)
	}
}

// request performs a request against a url
func (r *Registry) request(method string, url string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", strings.Join([]string{manifestv2.MediaTypeManifest, oci.MediaTypeImageManifest, oci.MediaTypeImageIndex, mediaTypeDockerManifestList}, ","))
	resp, err := r.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		return nil, fmt.Errorf("%s %s returned status %s", method, url, resp.Status)
	}

	return resp, nil
}
