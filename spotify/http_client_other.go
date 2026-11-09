//go:build !windows

package spotify

import (
	"net/http"
	"net/url"
)

func spotifySystemProxy(_ *http.Request) (*url.URL, error) {
	return nil, nil
}
