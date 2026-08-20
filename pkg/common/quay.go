package common

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

// ParseQuayDuration parses a quay.io expiration duration string such as "2w",
// "3d" or "1h" into a time.Duration. These are the duration formats accepted
// by the quay.expires-after label.
func ParseQuayDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid quay duration %q: too short", s)
	}
	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid quay duration %q: %w", s, err)
	}
	if num <= 0 {
		return 0, fmt.Errorf("invalid quay duration %q: value must be positive", s)
	}
	switch unit {
	case 'h':
		return time.Duration(num) * time.Hour, nil
	case 'd':
		return time.Duration(num) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(num) * 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid quay duration %q: unknown unit %q (expected h, d, or w)", s, string(unit))
	}
}

// IsQuayRegistry returns true if the given image reference targets a quay.io
// registry. Only the quay.io public registry is matched; self-hosted Quay
// instances are not detected.
func IsQuayRegistry(imageRef string) bool {
	imageName := GetImageName(imageRef)
	if imageName == "" {
		return false
	}
	registry, _, _ := strings.Cut(imageName, "/")
	return registry == "quay.io"
}

// QuayAPIClient can call the quay.io REST API. It is an interface so that
// tests can substitute a fake without hitting the network.
type QuayAPIClient interface {
	SetTagExpiration(repoPath, tag string, expiration time.Time) error
}

// quayAPIClient is the production implementation of QuayAPIClient. It sends
// real HTTP requests to the quay.io API.
type quayAPIClient struct {
	registryDomain string
	authToken      string
	httpClient     *http.Client
}

// NewQuayAPIClient creates a QuayAPIClient targeting the given registry
// domain (typically "quay.io") and authenticating with authToken (the
// base64-encoded "user:password" value from the docker config file).
func NewQuayAPIClient(registryDomain, authToken string) QuayAPIClient {
	return &quayAPIClient{
		registryDomain: registryDomain,
		authToken:      authToken,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
	}
}

// SetTagExpiration sets the expiration time on a tag in a quay.io repository
// via the REST API: PUT /api/v1/repository/{namespace}/{name}/tag/{tag}.
//
// repoPath is the repository path without the registry domain, e.g.
// "myorg/myrepo".
func (c *quayAPIClient) SetTagExpiration(repoPath, tag string, expiration time.Time) error {
	apiURL := fmt.Sprintf("https://%s/api/v1/repository/%s/tag/%s",
		c.registryDomain, repoPath, tag)

	body := fmt.Sprintf(`{"expiration": %d}`, expiration.Unix())

	req, err := http.NewRequest("PUT", apiURL, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating quay API request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("quay API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("quay API returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// getAuthFile returns the path to the docker config JSON file, respecting
// the DOCKER_CONFIG environment variable (which points to the directory
// containing config.json). Falls back to ~/.docker/config.json.
func getAuthFile() string {
	if dockerConfig := os.Getenv("DOCKER_CONFIG"); dockerConfig != "" {
		return filepath.Join(dockerConfig, "config.json")
	}
	return GetDefaultAuthFile()
}

// SetQuayTagExpiration sets tag expiration for index tags on quay.io via the
// REST API. OCI indexes have no image config, so quay cannot read the
// quay.expires-after label from them — this API call is the workaround.
//
// imageRef is the full image reference including registry, e.g.
// "quay.io/myorg/myrepo:mytag".
// expiresAfter is a quay-style duration string, e.g. "2w".
// tags is the list of tags to set expiration on.
//
// Failures are logged as warnings and do not return errors — tag expiration
// via the API is best-effort, mirroring the best-effort nature of the
// quay.expires-after label.
func SetQuayTagExpiration(imageRef, expiresAfter string, tags []string) {
	if !IsQuayRegistry(imageRef) {
		l.Logger.Debug("Skipping quay tag expiration: not a quay.io registry")
		return
	}

	duration, err := ParseQuayDuration(expiresAfter)
	if err != nil {
		l.Logger.Warnf("Skipping quay tag expiration: %v", err)
		return
	}
	expiration := time.Now().Add(duration)

	authFile := getAuthFile()
	auth, err := SelectRegistryAuth(imageRef, authFile)
	if err != nil {
		l.Logger.Warnf("Skipping quay tag expiration: could not read registry auth: %v", err)
		return
	}

	imageName := GetImageName(imageRef)
	// imageName is e.g. "quay.io/myorg/myrepo", strip the registry domain
	parts := strings.SplitN(imageName, "/", 2)
	if len(parts) < 2 || parts[1] == "" {
		l.Logger.Warnf("Skipping quay tag expiration: cannot determine repository path from %q", imageName)
		return
	}
	repoPath := parts[1]

	client := NewQuayAPIClient(parts[0], auth.Token)

	for _, tag := range tags {
		l.Logger.Infof("Setting quay tag expiration on index tag %q (expires %s)", tag, expiration.Format(time.RFC3339))
		if err := client.SetTagExpiration(repoPath, tag, expiration); err != nil {
			l.Logger.Warnf("Failed to set quay tag expiration on %q: %v", tag, err)
		}
	}
}
