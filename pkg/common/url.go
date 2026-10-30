package common

import "net/url"

// SanitizeURL removes credentials from a URL for safe logging.
func SanitizeURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		// Unparseable, so reveal nothing.
		return "<invalid URL>"
	}
	if parsed.User != nil {
		// User contains username and password information.
		parsed.User = url.User("***")
	}
	return parsed.String()
}
