package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/provider"
)

const (
	defaultMarket          = "US"
	defaultRequestTimeout  = 20 * time.Second
	defaultTokenSafetyTime = 30 * time.Second
	maxAPIResponseSize     = 16 << 20
)

var (
	spotifyAPIBase               = "https://api.spotify.com/v1"
	spotifyAccountsBase          = "https://accounts.spotify.com"
	spotifyOpenBase              = "https://open.spotify.com"
	spotifyEmbedBase             = "https://embed.spotify.com"
	spotifyHTTPClient            = newSpotifyHTTPClient(defaultRequestTimeout)
	errSpotifyMissingCredentials = errors.New(
		"spotify requires client_id and client_secret, or an access_token, configured in the spotify settings",
	)
)

var (
	_ provider.FullMusicProvider           = (*Spotify)(nil)
	_ provider.RankingProvider             = (*Spotify)(nil)
	_ provider.UserPlaylistProvider        = (*Spotify)(nil)
	_ provider.RecommendedPlaylistProvider = (*Spotify)(nil)
)

// Spotify implements the public Spotify Web API catalog and, when an OAuth
// refresh token is configured, the current user's playlists.
type Spotify struct {
	clientID     string
	clientSecret string
	refreshToken string
	accessToken  string
	market       string
	tokenIsUser  bool

	tokenMu     sync.Mutex
	tokenExpiry time.Time
}

type spotifyConfig struct {
	clientID     string
	clientSecret string
	refreshToken string
	accessToken  string
	market       string
}

type spotifyTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

type spotifyHTTPError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *spotifyHTTPError) Error() string {
	switch {
	case e.Message != "":
		return fmt.Sprintf("spotify api: status %d: %s", e.StatusCode, e.Message)
	case e.Body != "":
		return fmt.Sprintf("spotify api: status %d: %s", e.StatusCode, e.Body)
	default:
		return fmt.Sprintf("spotify api: status %d", e.StatusCode)
	}
}

func New(cookie string) *Spotify {
	cfg := parseSpotifyConfig(cookie)
	cfg = mergeSpotifyEnvironment(cfg)

	market := strings.ToUpper(strings.TrimSpace(cfg.market))
	if len(market) != 2 {
		market = defaultMarket
	}

	return &Spotify{
		clientID:     strings.TrimSpace(cfg.clientID),
		clientSecret: strings.TrimSpace(cfg.clientSecret),
		refreshToken: strings.TrimSpace(cfg.refreshToken),
		accessToken:  strings.TrimSpace(cfg.accessToken),
		market:       market,
		tokenIsUser:  strings.TrimSpace(cfg.accessToken) != "",
	}
}

var defaultSpotify = New("")

func (s *Spotify) marketOrDefault() string {
	if s == nil || len(strings.TrimSpace(s.market)) != 2 {
		return defaultMarket
	}
	return strings.ToUpper(strings.TrimSpace(s.market))
}

func parseSpotifyConfig(raw string) spotifyConfig {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return spotifyConfig{}
	}

	values := make(map[string]string)
	if strings.HasPrefix(raw, "{") {
		var payload map[string]any
		if err := json.Unmarshal([]byte(raw), &payload); err == nil {
			for key, value := range payload {
				values[normalizeSpotifyConfigKey(key)] = fmt.Sprint(value)
			}
		}
	}

	if len(values) == 0 {
		replacer := strings.NewReplacer("&", ";", "\n", ";", "\r", ";")
		for _, item := range strings.Split(replacer.Replace(raw), ";") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			parts := strings.SplitN(item, "=", 2)
			if len(parts) != 2 {
				continue
			}
			values[normalizeSpotifyConfigKey(parts[0])] = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		}
	}

	return spotifyConfig{
		clientID: firstSpotifyConfigValue(values,
			"client_id", "clientid", "spotify_client_id", "spotifyclientid"),
		clientSecret: firstSpotifyConfigValue(values,
			"client_secret", "clientsecret", "spotify_client_secret", "spotifyclientsecret"),
		refreshToken: firstSpotifyConfigValue(values,
			"refresh_token", "refreshtoken", "spotify_refresh_token", "spotifyrefreshtoken"),
		accessToken: firstSpotifyConfigValue(values,
			"access_token", "accesstoken", "bearer_token", "bearertoken", "token"),
		market: firstSpotifyConfigValue(values,
			"market", "country", "spotify_market", "spotifymarket"),
	}
}

func normalizeSpotifyConfigKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.NewReplacer("-", "_", " ", "_").Replace(key)
	return key
}

func firstSpotifyConfigValue(values map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(values[normalizeSpotifyConfigKey(key)]); value != "" {
			return value
		}
	}
	return ""
}

func mergeSpotifyEnvironment(cfg spotifyConfig) spotifyConfig {
	if cfg.clientID == "" {
		cfg.clientID = strings.TrimSpace(os.Getenv("SPOTIFY_CLIENT_ID"))
	}
	if cfg.clientSecret == "" {
		cfg.clientSecret = strings.TrimSpace(os.Getenv("SPOTIFY_CLIENT_SECRET"))
	}
	if cfg.refreshToken == "" {
		cfg.refreshToken = strings.TrimSpace(os.Getenv("SPOTIFY_REFRESH_TOKEN"))
	}
	if cfg.accessToken == "" {
		cfg.accessToken = strings.TrimSpace(os.Getenv("SPOTIFY_ACCESS_TOKEN"))
	}
	if cfg.market == "" {
		cfg.market = strings.TrimSpace(os.Getenv("SPOTIFY_MARKET"))
	}
	return cfg
}

func (s *Spotify) ensureAccessToken(ctx context.Context, userOnly bool) (string, error) {
	if s == nil {
		return "", errors.New("spotify client is nil")
	}

	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()

	if s.accessToken != "" &&
		(!userOnly || s.tokenIsUser) &&
		(s.tokenExpiry.IsZero() || time.Now().Add(defaultTokenSafetyTime).Before(s.tokenExpiry)) {
		return s.accessToken, nil
	}

	if s.refreshToken != "" {
		token, err := s.fetchAccessToken(ctx, "refresh_token")
		if err == nil {
			return token, nil
		}
		if userOnly {
			return "", err
		}
	}

	if userOnly {
		return "", errors.New("spotify user playlists require access_token or refresh_token with playlist-read-private scope")
	}
	if s.clientID == "" || s.clientSecret == "" {
		return "", errSpotifyMissingCredentials
	}
	return s.fetchAccessToken(ctx, "client_credentials")
}

func (s *Spotify) fetchAccessToken(ctx context.Context, grantType string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", grantType)
	switch grantType {
	case "refresh_token":
		form.Set("refresh_token", s.refreshToken)
		if s.clientSecret == "" {
			form.Set("client_id", s.clientID)
		}
	case "client_credentials":
		if s.clientID == "" || s.clientSecret == "" {
			return "", errors.New("spotify client credentials are not configured")
		}
	default:
		return "", fmt.Errorf("unsupported spotify grant type: %s", grantType)
	}

	endpoint := strings.TrimRight(spotifyAccountsBase, "/") + "/api/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if s.clientID != "" && s.clientSecret != "" {
		req.SetBasicAuth(s.clientID, s.clientSecret)
	}

	resp, err := spotifyHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("spotify token request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAPIResponseSize))
	if err != nil {
		return "", fmt.Errorf("spotify token response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", newSpotifyHTTPError(resp.StatusCode, body)
	}

	var token spotifyTokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return "", fmt.Errorf("spotify token json: %w", err)
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		return "", errors.New("spotify token response did not contain access_token")
	}

	s.accessToken = token.AccessToken
	s.tokenIsUser = grantType == "refresh_token"
	expiresIn := token.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	s.tokenExpiry = time.Now().Add(time.Duration(expiresIn) * time.Second)
	if token.RefreshToken != "" {
		s.refreshToken = token.RefreshToken
	}
	return s.accessToken, nil
}

func (s *Spotify) invalidateAccessToken(token string) {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	if token == "" || s.accessToken == token {
		s.accessToken = ""
		s.tokenExpiry = time.Time{}
		s.tokenIsUser = false
	}
}

func (s *Spotify) getJSON(ctx context.Context, path string, query url.Values, userOnly bool, target any) error {
	if s == nil {
		return errors.New("spotify client is nil")
	}

	authRetried := false
	rateLimitRetried := false
	for attempt := 0; attempt < 3; attempt++ {
		token, err := s.ensureAccessToken(ctx, userOnly)
		if err != nil {
			return err
		}

		requestURL := strings.TrimRight(spotifyAPIBase, "/") + "/" + strings.TrimLeft(path, "/")
		if len(query) > 0 {
			requestURL += "?" + query.Encode()
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "go-music-dl-spotify/1.0")

		resp, err := spotifyHTTPClient.Do(req)
		if err != nil {
			return fmt.Errorf("spotify request: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxAPIResponseSize))
		closeErr := resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("spotify response: %w", readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("spotify response close: %w", closeErr)
		}

		if resp.StatusCode == http.StatusUnauthorized && !authRetried {
			authRetried = true
			s.invalidateAccessToken(token)
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests && !rateLimitRetried {
			rateLimitRetried = true
			if err := waitSpotifyRetry(ctx, resp); err != nil {
				return err
			}
			continue
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return newSpotifyHTTPError(resp.StatusCode, body)
		}
		if target == nil {
			return nil
		}
		if err := json.Unmarshal(body, target); err != nil {
			return fmt.Errorf("spotify json: %w", err)
		}
		return nil
	}
	return errors.New("spotify request failed after authentication retry")
}

func waitSpotifyRetry(ctx context.Context, resp *http.Response) error {
	timer := time.NewTimer(spotifyRetryDelay(resp))
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func spotifyRetryDelay(resp *http.Response) time.Duration {
	raw := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if raw == "" {
		return time.Second
	}
	if seconds, err := strconv.Atoi(raw); err == nil {
		if seconds < 0 {
			seconds = 1
		}
		if seconds > 5 {
			seconds = 5
		}
		return time.Duration(seconds) * time.Second
	}
	if retryAt, err := http.ParseTime(raw); err == nil {
		delay := time.Until(retryAt)
		if delay <= 0 {
			return time.Second
		}
		if delay > 5*time.Second {
			delay = 5 * time.Second
		}
		return delay
	}
	return time.Second
}

func newSpotifyHTTPError(statusCode int, body []byte) error {
	message := ""
	var payload struct {
		Error struct {
			Status  int    `json:"status"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &payload) == nil {
		message = strings.TrimSpace(payload.Error.Message)
	}
	text := strings.TrimSpace(string(body))
	if len(text) > 500 {
		text = text[:500]
	}
	return &spotifyHTTPError{
		StatusCode: statusCode,
		Message:    message,
		Body:       text,
	}
}

type spotifyImage struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type spotifyArtist struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	ExternalURLs map[string]string `json:"external_urls"`
}

type spotifyAlbum struct {
	ID                   string            `json:"id"`
	Name                 string            `json:"name"`
	AlbumType            string            `json:"album_type"`
	TotalTracks          int               `json:"total_tracks"`
	ReleaseDate          string            `json:"release_date"`
	ReleaseDatePrecision string            `json:"release_date_precision"`
	Label                string            `json:"label"`
	Artists              []spotifyArtist   `json:"artists"`
	Images               []spotifyImage    `json:"images"`
	ExternalURLs         map[string]string `json:"external_urls"`
}

type spotifyTrack struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	DurationMS   int               `json:"duration_ms"`
	TrackNumber  int               `json:"track_number"`
	DiscNumber   int               `json:"disc_number"`
	Explicit     bool              `json:"explicit"`
	PreviewURL   string            `json:"preview_url"`
	ExternalURLs map[string]string `json:"external_urls"`
	Artists      []spotifyArtist   `json:"artists"`
	Album        spotifyAlbum      `json:"album"`
	ExternalIDs  map[string]string `json:"external_ids"`
	IsPlayable   *bool             `json:"is_playable"`
	Type         string            `json:"type"`
}

type spotifyPlaylistOwner struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type spotifyPlaylist struct {
	ID           string                `json:"id"`
	Name         string                `json:"name"`
	Description  string                `json:"description"`
	Images       []spotifyImage        `json:"images"`
	Owner        spotifyPlaylistOwner  `json:"owner"`
	SnapshotID   string                `json:"snapshot_id"`
	ExternalURLs map[string]string     `json:"external_urls"`
	Tracks       spotifyPlaylistTracks `json:"tracks"`
	Items        spotifyPlaylistTracks `json:"items"`
}

type spotifyPlaylistTracks struct {
	Total int                   `json:"total"`
	Items []spotifyPlaylistItem `json:"items"`
	Next  string                `json:"next"`
}

func (playlist spotifyPlaylist) trackCount() int {
	if playlist.Tracks.Total > 0 {
		return playlist.Tracks.Total
	}
	return playlist.Items.Total
}

type spotifyPlaylistItem struct {
	AddedAt string        `json:"added_at"`
	Track   *spotifyTrack `json:"track"`
	Item    *spotifyTrack `json:"item"`
}

func (item spotifyPlaylistItem) track() *spotifyTrack {
	if item.Track != nil {
		return item.Track
	}
	return item.Item
}

type spotifyPaging[T any] struct {
	Items  []T    `json:"items"`
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
	Next   string `json:"next"`
}

type spotifySearchResponse struct {
	Tracks    spotifyPaging[spotifyTrack]    `json:"tracks"`
	Albums    spotifyPaging[spotifyAlbum]    `json:"albums"`
	Playlists spotifyPaging[spotifyPlaylist] `json:"playlists"`
}

type spotifyCategory struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Icons []spotifyImage `json:"icons"`
}

func spotifyCover(images []spotifyImage) string {
	bestURL := ""
	bestWidth := -1
	for _, image := range images {
		if strings.TrimSpace(image.URL) == "" {
			continue
		}
		width := image.Width
		if width <= 0 {
			width = image.Height
		}
		if width > bestWidth {
			bestURL = strings.TrimSpace(image.URL)
			bestWidth = width
		}
	}
	return bestURL
}

func spotifyArtists(artists []spotifyArtist) string {
	names := make([]string, 0, len(artists))
	seen := make(map[string]struct{}, len(artists))
	for _, artist := range artists {
		name := strings.TrimSpace(artist.Name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

func spotifyPlaylistFromAPI(playlist spotifyPlaylist) model.Playlist {
	description := strings.TrimSpace(playlist.Description)
	return model.Playlist{
		Source:      "spotify",
		ID:          strings.TrimSpace(playlist.ID),
		Name:        strings.TrimSpace(playlist.Name),
		Cover:       spotifyCover(playlist.Images),
		TrackCount:  playlist.trackCount(),
		Creator:     firstSpotifyNonEmpty(playlist.Owner.DisplayName, playlist.Owner.ID),
		Description: description,
		Link:        firstSpotifyNonEmpty(playlist.ExternalURLs["spotify"], spotifyPlaylistLink(playlist.ID)),
		Extra: map[string]string{
			"type":        "playlist",
			"spotify_id":  strings.TrimSpace(playlist.ID),
			"owner_id":    strings.TrimSpace(playlist.Owner.ID),
			"snapshot_id": strings.TrimSpace(playlist.SnapshotID),
		},
	}
}

func spotifyAlbumFromAPI(album spotifyAlbum) model.Playlist {
	return model.Playlist{
		Source:      "spotify",
		ID:          strings.TrimSpace(album.ID),
		Name:        strings.TrimSpace(album.Name),
		Cover:       spotifyCover(album.Images),
		TrackCount:  album.TotalTracks,
		Creator:     spotifyArtists(album.Artists),
		Description: strings.TrimSpace(album.Label),
		Link:        firstSpotifyNonEmpty(album.ExternalURLs["spotify"], spotifyAlbumLink(album.ID)),
		Extra: map[string]string{
			"type":         "album",
			"spotify_id":   strings.TrimSpace(album.ID),
			"album_type":   strings.TrimSpace(album.AlbumType),
			"release_date": strings.TrimSpace(album.ReleaseDate),
		},
	}
}

func spotifySongFromTrack(track spotifyTrack, fallbackAlbum *spotifyAlbum) model.Song {
	album := track.Album
	if fallbackAlbum != nil {
		if album.ID == "" {
			album.ID = fallbackAlbum.ID
		}
		if album.Name == "" {
			album.Name = fallbackAlbum.Name
		}
		if len(album.Images) == 0 {
			album.Images = fallbackAlbum.Images
		}
		if len(album.Artists) == 0 {
			album.Artists = fallbackAlbum.Artists
		}
	}

	artist := spotifyArtists(track.Artists)
	if artist == "" {
		artist = spotifyArtists(album.Artists)
	}

	extra := map[string]string{
		"type":       "track",
		"spotify_id": strings.TrimSpace(track.ID),
	}
	if track.TrackNumber > 0 {
		extra["track_number"] = strconv.Itoa(track.TrackNumber)
	}
	if track.DiscNumber > 0 {
		extra["disc_number"] = strconv.Itoa(track.DiscNumber)
	}
	if track.Explicit {
		extra["explicit"] = "true"
	}
	if isrc := strings.TrimSpace(track.ExternalIDs["isrc"]); isrc != "" {
		extra["isrc"] = isrc
	}
	extra["preview_only"] = "true"

	return model.Song{
		Source:   "spotify",
		ID:       strings.TrimSpace(track.ID),
		Name:     strings.TrimSpace(track.Name),
		Artist:   artist,
		Album:    strings.TrimSpace(album.Name),
		AlbumID:  strings.TrimSpace(album.ID),
		Duration: track.DurationMS / 1000,
		Cover:    spotifyCover(album.Images),
		URL:      strings.TrimSpace(track.PreviewURL),
		Ext:      "mp3",
		Link:     firstSpotifyNonEmpty(track.ExternalURLs["spotify"], spotifyTrackLink(track.ID)),
		Extra:    extra,
	}
}

func firstSpotifyNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func spotifyTrackLink(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	return "https://open.spotify.com/track/" + id
}

func spotifyAlbumLink(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	return "https://open.spotify.com/album/" + id
}

func spotifyPlaylistLink(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	return "https://open.spotify.com/playlist/" + id
}
