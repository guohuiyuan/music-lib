package spotify

import (
	"fmt"
	"net/url"
	"strings"
)

func extractSpotifyID(raw, expectedType string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("spotify %s id is empty", expectedType)
	}

	if strings.HasPrefix(strings.ToLower(raw), "spotify:") {
		parts := strings.Split(raw, ":")
		if len(parts) < 3 {
			return "", fmt.Errorf("invalid spotify %s uri: %s", expectedType, raw)
		}
		idType := strings.ToLower(strings.TrimSpace(parts[1]))
		id := strings.TrimSpace(parts[2])
		if idType != expectedType {
			return "", fmt.Errorf("expected spotify %s, got %s", expectedType, idType)
		}
		if id == "" {
			return "", fmt.Errorf("spotify %s id is empty", expectedType)
		}
		return id, nil
	}

	if !strings.ContainsAny(raw, "/:?") {
		return raw, nil
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid spotify %s link: %w", expectedType, err)
	}
	host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
	if host != "open.spotify.com" && host != "play.spotify.com" {
		return "", fmt.Errorf("invalid spotify %s link: %s", expectedType, raw)
	}

	segments := make([]string, 0, 4)
	for _, segment := range strings.Split(parsed.Path, "/") {
		segment = strings.TrimSpace(segment)
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	for len(segments) > 0 && (segments[0] == "embed" || strings.HasPrefix(segments[0], "intl-")) {
		segments = segments[1:]
	}
	if len(segments) < 2 {
		return "", fmt.Errorf("invalid spotify %s link: %s", expectedType, raw)
	}
	idType := strings.ToLower(segments[0])
	if idType != expectedType {
		return "", fmt.Errorf("expected spotify %s, got %s", expectedType, idType)
	}
	id := strings.TrimSpace(segments[1])
	if id == "" {
		return "", fmt.Errorf("spotify %s id is empty", expectedType)
	}
	return id, nil
}

func normalizeSpotifyPage(page, limit, defaultLimit, maxLimit int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	return (page - 1) * limit, limit
}
