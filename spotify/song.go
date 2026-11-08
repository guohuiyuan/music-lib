package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
)

const spotifySearchLimit = 30

var lrclibAPIBase = "https://lrclib.net/api"

func Search(keyword string) ([]model.Song, error) {
	return defaultSpotify.Search(keyword)
}

func Parse(link string) (*model.Song, error) {
	return defaultSpotify.Parse(link)
}

func GetDownloadURL(song *model.Song) (string, error) {
	return defaultSpotify.GetDownloadURL(song)
}

func GetLyrics(song *model.Song) (string, error) {
	return defaultSpotify.GetLyrics(song)
}

func (s *Spotify) Search(keyword string) ([]model.Song, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, errors.New("spotify search keyword is empty")
	}

	query := url.Values{}
	query.Set("q", keyword)
	query.Set("type", "track")
	query.Set("limit", strconv.Itoa(spotifySearchLimit))
	query.Set("market", s.marketOrDefault())

	var response spotifySearchResponse
	if err := s.getJSON(context.Background(), "/search", query, false, &response); err != nil {
		return nil, fmt.Errorf("spotify search: %w", err)
	}

	songs := make([]model.Song, 0, len(response.Tracks.Items))
	for _, track := range response.Tracks.Items {
		if strings.TrimSpace(track.ID) == "" {
			continue
		}
		songs = append(songs, spotifySongFromTrack(track, nil))
	}
	return songs, nil
}

func (s *Spotify) Parse(link string) (*model.Song, error) {
	id, err := extractSpotifyID(link, "track")
	if err != nil {
		return nil, err
	}
	if !s.hasCatalogCredentials() {
		song, fallbackErr := s.fetchPublicTrack(id)
		if fallbackErr != nil {
			return nil, fmt.Errorf(
				"spotify public track %s: %w",
				id,
				fallbackErr,
			)
		}
		return song, nil
	}

	track, err := s.getTrack(id)
	if err != nil {
		if song, fallbackErr := s.fetchPublicTrack(id); fallbackErr == nil {
			return song, nil
		}
		return nil, err
	}
	song := spotifySongFromTrack(*track, nil)
	return &song, nil
}

func (s *Spotify) hasCatalogCredentials() bool {
	if s == nil {
		return false
	}
	return strings.TrimSpace(s.accessToken) != "" ||
		strings.TrimSpace(s.refreshToken) != "" ||
		(strings.TrimSpace(s.clientID) != "" &&
			strings.TrimSpace(s.clientSecret) != "")
}

func (s *Spotify) getTrack(id string) (*spotifyTrack, error) {
	query := url.Values{}
	query.Set("market", s.marketOrDefault())

	var track spotifyTrack
	if err := s.getJSON(context.Background(), "/tracks/"+url.PathEscape(id), query, false, &track); err != nil {
		return nil, fmt.Errorf("spotify track %s: %w", id, err)
	}
	if strings.TrimSpace(track.ID) == "" {
		return nil, fmt.Errorf("spotify track %s was not found", id)
	}
	return &track, nil
}

// GetDownloadURL returns Spotify's official preview URL. Spotify's full tracks
// are DRM protected and are intentionally not exposed as downloadable files.
func (s *Spotify) GetDownloadURL(song *model.Song) (string, error) {
	if song == nil {
		return "", errors.New("spotify song is nil")
	}
	if preview := strings.TrimSpace(song.URL); preview != "" {
		return preview, nil
	}

	id := strings.TrimSpace(song.ID)
	if id == "" {
		var err error
		id, err = extractSpotifyID(song.Link, "track")
		if err != nil {
			return "", errors.New("spotify song id is empty")
		}
	}

	var apiErr error
	if s.hasCatalogCredentials() {
		track, err := s.getTrack(id)
		if err == nil {
			preview := strings.TrimSpace(track.PreviewURL)
			if preview != "" {
				return preview, nil
			}
			apiErr = errors.New(
				"spotify does not provide an API preview URL for this track",
			)
		} else {
			apiErr = err
		}
	}

	publicSong, fallbackErr := s.fetchPublicTrack(id)
	if fallbackErr == nil {
		if preview := strings.TrimSpace(publicSong.URL); preview != "" {
			return preview, nil
		}
		fallbackErr = errors.New(
			"spotify public embed does not provide a preview URL for this track",
		)
	}
	if apiErr != nil {
		return "", fmt.Errorf(
			"spotify track %s: %w (public embed fallback: %v)",
			id,
			apiErr,
			fallbackErr,
		)
	}
	return "", fmt.Errorf(
		"spotify track %s: %w; full tracks are DRM protected",
		id,
		fallbackErr,
	)
}

func (s *Spotify) fetchPublicTrack(id string) (*model.Song, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()
	entity, err := s.fetchSpotifyEmbedEntityContextWithTimeout(
		ctx,
		"track",
		id,
		spotifyTrackEmbedTimeout,
	)
	if err != nil {
		return nil, err
	}
	song, err := spotifySongFromEmbed(*entity)
	if err != nil {
		return nil, err
	}
	return &song, nil
}

func (s *Spotify) GetLyrics(song *model.Song) (string, error) {
	if song == nil {
		return "", errors.New("spotify song is nil")
	}

	metadata := *song
	if strings.TrimSpace(metadata.Name) == "" || strings.TrimSpace(metadata.Artist) == "" {
		id := strings.TrimSpace(song.ID)
		if id == "" {
			var err error
			id, err = extractSpotifyID(song.Link, "track")
			if err != nil {
				return "", errors.New("spotify lyrics require song id, name, and artist")
			}
		}
		track, err := s.getTrack(id)
		if err != nil {
			return "", err
		}
		metadata = spotifySongFromTrack(*track, nil)
	}
	if strings.TrimSpace(metadata.Name) == "" || strings.TrimSpace(metadata.Artist) == "" {
		return "", errors.New("spotify lyrics require track name and artist")
	}

	query := url.Values{}
	query.Set("track_name", metadata.Name)
	query.Set("artist_name", metadata.Artist)
	if metadata.Album != "" {
		query.Set("album_name", metadata.Album)
	}
	if metadata.Duration > 0 {
		query.Set("duration", strconv.Itoa(metadata.Duration))
	}

	var exact lrclibLyrics
	status, body, err := spotifyPublicGet(lrclibAPIBase+"/get", query)
	if err != nil {
		return "", fmt.Errorf("spotify lyrics: %w", err)
	}
	if status == http.StatusOK {
		if err := json.Unmarshal(body, &exact); err != nil {
			return "", fmt.Errorf("spotify lyrics json: %w", err)
		}
		if lyrics := exact.text(); lyrics != "" {
			return lyrics, nil
		}
	}
	if status != http.StatusNotFound && status != http.StatusOK {
		return "", fmt.Errorf("spotify lyrics: LRCLIB returned status %d", status)
	}

	searchQuery := url.Values{}
	searchQuery.Set("track_name", metadata.Name)
	searchQuery.Set("artist_name", metadata.Artist)
	status, body, err = spotifyPublicGet(lrclibAPIBase+"/search", searchQuery)
	if err != nil {
		return "", fmt.Errorf("spotify lyrics search: %w", err)
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("spotify lyrics search: LRCLIB returned status %d", status)
	}

	var candidates []lrclibLyrics
	if err := json.Unmarshal(body, &candidates); err != nil {
		return "", fmt.Errorf("spotify lyrics search json: %w", err)
	}
	for _, candidate := range candidates {
		if lyrics := candidate.text(); lyrics != "" {
			return lyrics, nil
		}
	}
	return "", nil
}

type lrclibLyrics struct {
	SyncedLyrics string `json:"syncedLyrics"`
	PlainLyrics  string `json:"plainLyrics"`
}

func (lyrics lrclibLyrics) text() string {
	if text := strings.TrimSpace(lyrics.SyncedLyrics); text != "" {
		return text
	}
	return strings.TrimSpace(lyrics.PlainLyrics)
}

func spotifyPublicGet(endpoint string, query url.Values) (int, []byte, error) {
	return spotifyPublicGetWithAccept(endpoint, query, "application/json")
}

func spotifyPublicGetWithAccept(
	endpoint string,
	query url.Values,
	accept string,
) (int, []byte, error) {
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()
	return spotifyPublicGetContextAccept(ctx, endpoint, nil, accept)
}

func spotifyPublicGetContextAccept(
	ctx context.Context,
	endpoint string,
	query url.Values,
	accept string,
) (int, []byte, error) {
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", "go-music-dl-spotify/1.0")

	resp, err := spotifyHTTPClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxAPIResponseSize))
	closeErr := resp.Body.Close()
	if readErr != nil {
		return 0, nil, readErr
	}
	if closeErr != nil {
		return 0, nil, closeErr
	}
	return resp.StatusCode, body, nil
}
