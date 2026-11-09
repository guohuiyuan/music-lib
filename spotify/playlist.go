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
	"time"

	"github.com/guohuiyuan/music-lib/model"
)

const (
	spotifyPlaylistSearchLimit = 20
	spotifyPlaylistPageSize    = 50
	spotifyTrackEmbedTimeout   = 8 * time.Second
)

type spotifyEmbedPageResponse struct {
	Props struct {
		PageProps struct {
			Status int `json:"status"`
			State  struct {
				Data struct {
					Entity spotifyEmbedEntity `json:"entity"`
				} `json:"data"`
			} `json:"state"`
		} `json:"pageProps"`
	} `json:"props"`
}

type spotifyEmbedEntity struct {
	Type           string               `json:"type"`
	URI            string               `json:"uri"`
	ID             string               `json:"id"`
	Name           string               `json:"name"`
	Title          string               `json:"title"`
	Subtitle       string               `json:"subtitle"`
	Artists        []spotifyArtist      `json:"artists"`
	Duration       int                  `json:"duration"`
	ReleaseDate    string               `json:"releaseDate"`
	AudioPreview   *spotifyEmbedPreview `json:"audioPreview"`
	CoverArt       spotifyEmbedArt      `json:"coverArt"`
	VisualIdentity struct {
		Image []spotifyImage `json:"image"`
	} `json:"visualIdentity"`
	TrackList []spotifyEmbedTrack `json:"trackList"`
}

type spotifyEmbedArt struct {
	Sources []spotifyImage `json:"sources"`
}

type spotifyEmbedTrack struct {
	URI          string               `json:"uri"`
	Title        string               `json:"title"`
	Subtitle     string               `json:"subtitle"`
	Duration     int                  `json:"duration"`
	IsPlayable   bool                 `json:"isPlayable"`
	IsExplicit   bool                 `json:"isExplicit"`
	AudioPreview *spotifyEmbedPreview `json:"audioPreview"`
}

type spotifyEmbedPreview struct {
	Format string `json:"format"`
	URL    string `json:"url"`
}

func SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return defaultSpotify.SearchPlaylist(keyword)
}

func GetPlaylistSongs(id string) ([]model.Song, error) {
	return defaultSpotify.GetPlaylistSongs(id)
}

func ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	return defaultSpotify.ParsePlaylist(link)
}

func (s *Spotify) SearchPlaylist(keyword string) ([]model.Playlist, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, errors.New("spotify playlist search keyword is empty")
	}
	if !s.hasCatalogAuthentication() {
		playlists, err := s.searchPlaylistsFromWebPlayer(
			context.Background(),
			keyword,
			spotifyPlaylistSearchLimit,
		)
		if err != nil {
			return nil, fmt.Errorf("spotify playlist search anonymous fallback: %w", err)
		}
		return playlists, nil
	}

	query := url.Values{}
	query.Set("q", keyword)
	query.Set("type", "playlist")
	query.Set("limit", strconv.Itoa(spotifyPlaylistSearchLimit))
	query.Set("market", s.marketOrDefault())

	var response spotifySearchResponse
	if err := s.getJSON(context.Background(), "/search", query, false, &response); err != nil {
		if spotifyAllowsAnonymousSearchFallback(err) {
			playlists, fallbackErr := s.searchPlaylistsFromWebPlayer(
				context.Background(),
				keyword,
				spotifyPlaylistSearchLimit,
			)
			if fallbackErr == nil {
				return playlists, nil
			}
			return nil, fmt.Errorf(
				"spotify playlist search: official API: %v; anonymous fallback: %w",
				err,
				fallbackErr,
			)
		}
		return nil, fmt.Errorf("spotify playlist search: %w", err)
	}

	playlists := make([]model.Playlist, 0, len(response.Playlists.Items))
	for _, playlist := range response.Playlists.Items {
		if strings.TrimSpace(playlist.ID) == "" {
			continue
		}
		playlists = append(playlists, spotifyPlaylistFromAPI(playlist))
	}
	return playlists, nil
}

func (s *Spotify) GetPlaylistSongs(id string) ([]model.Song, error) {
	playlistID, err := extractSpotifyID(id, "playlist")
	if err != nil {
		return nil, err
	}
	return s.fetchPlaylistTracks(playlistID)
}

func (s *Spotify) ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	playlistID, err := extractSpotifyID(link, "playlist")
	if err != nil {
		return nil, nil, err
	}
	playlist, err := s.fetchPlaylist(playlistID)
	if err != nil {
		return nil, nil, err
	}
	songs, err := s.fetchPlaylistTracks(playlistID)
	if err != nil {
		return nil, nil, err
	}

	result := spotifyPlaylistFromAPI(*playlist)
	if result.TrackCount <= 0 || result.TrackCount > len(songs) && len(songs) > 0 {
		result.TrackCount = len(songs)
	}
	return &result, songs, nil
}

func (s *Spotify) fetchPlaylist(id string) (*spotifyPlaylist, error) {
	if !s.hasCatalogAuthentication() {
		playlist, oembedErr := s.fetchPlaylistFromOEmbed(id)
		if oembedErr == nil {
			return playlist, nil
		}
		playlist, err := s.fetchPlaylistFromEmbed(id)
		if err != nil {
			return nil, fmt.Errorf(
				"spotify playlist %s public metadata: %w",
				id,
				errors.Join(oembedErr, err),
			)
		}
		return playlist, nil
	}

	query := url.Values{}
	query.Set("market", s.marketOrDefault())

	var playlist spotifyPlaylist
	if err := s.getJSON(context.Background(), "/playlists/"+url.PathEscape(id), query, false, &playlist); err != nil {
		if spotifyAllowsPublicFallback(err) {
			fallback, fallbackErr := s.fetchPlaylistFromEmbed(id)
			if fallbackErr == nil {
				return fallback, nil
			}
			return nil, fmt.Errorf(
				"spotify playlist %s: %w (public embed fallback: %v)",
				id,
				err,
				fallbackErr,
			)
		}
		return nil, fmt.Errorf("spotify playlist %s: %w", id, err)
	}
	if strings.TrimSpace(playlist.ID) == "" {
		return nil, fmt.Errorf("spotify playlist %s was not found", id)
	}
	return &playlist, nil
}

func (s *Spotify) fetchPlaylistTracks(id string) ([]model.Song, error) {
	if !s.hasCatalogAuthentication() {
		songs, err := s.fetchPlaylistTracksFromPublicAlternatives(id)
		if err != nil {
			return nil, fmt.Errorf(
				"spotify playlist tracks %s public fallback: %w",
				id,
				err,
			)
		}
		return songs, nil
	}

	songs := make([]model.Song, 0)
	endpoint := "items"
	for offset := 0; ; {
		query := url.Values{}
		query.Set("limit", strconv.Itoa(spotifyPlaylistPageSize))
		query.Set("offset", strconv.Itoa(offset))
		query.Set("market", s.marketOrDefault())
		query.Set("additional_types", "track")

		var page spotifyPaging[spotifyPlaylistItem]
		path := "/playlists/" + url.PathEscape(id) + "/" + endpoint
		if err := s.getJSON(context.Background(), path, query, false, &page); err != nil {
			// Spotify renamed the endpoint to /items. Keep the old endpoint as
			// a fallback for applications pinned to the legacy Web API shape.
			if offset == 0 && endpoint == "items" && isSpotifyEndpointUnavailable(err) {
				endpoint = "tracks"
				continue
			}
			if spotifyAllowsPublicFallback(err) {
				fallbackSongs, fallbackErr := s.fetchPlaylistTracksFromEmbed(id)
				if fallbackErr == nil {
					return fallbackSongs, nil
				}
				return nil, fmt.Errorf(
					"spotify playlist tracks %s: %w (public embed fallback: %v)",
					id,
					err,
					fallbackErr,
				)
			}
			return nil, fmt.Errorf("spotify playlist tracks %s: %w", id, err)
		}
		for _, item := range page.Items {
			track := item.track()
			if track == nil || strings.TrimSpace(track.ID) == "" {
				continue
			}
			songs = append(songs, spotifySongFromTrack(*track, nil))
		}
		if len(page.Items) == 0 || len(songs) >= page.Total || page.Next == "" {
			break
		}
		offset += len(page.Items)
	}
	return songs, nil
}

func (s *Spotify) hasCatalogAuthentication() bool {
	if s == nil {
		return false
	}
	return strings.TrimSpace(s.accessToken) != "" ||
		strings.TrimSpace(s.refreshToken) != "" ||
		(strings.TrimSpace(s.clientID) != "" && strings.TrimSpace(s.clientSecret) != "")
}

func spotifyAllowsPublicFallback(err error) bool {
	if errors.Is(err, errSpotifyMissingCredentials) {
		return true
	}
	var apiErr *spotifyHTTPError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.StatusCode {
	case http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusTooManyRequests:
		return true
	default:
		return false
	}
}

func (s *Spotify) fetchPlaylistFromEmbed(id string) (*spotifyPlaylist, error) {
	entity, err := s.fetchPlaylistEmbedEntity(id)
	if err != nil {
		return nil, err
	}
	playlist := spotifyPlaylist{
		ID:          firstSpotifyNonEmpty(entity.ID, id),
		Name:        firstSpotifyNonEmpty(entity.Name, entity.Title),
		Description: "Spotify public playlist",
		Images:      entity.CoverArt.Sources,
		Owner: spotifyPlaylistOwner{
			DisplayName: firstSpotifyNonEmpty(
				cleanSpotifyEmbedText(entity.Subtitle),
				"Spotify",
			),
		},
		ExternalURLs: map[string]string{
			"spotify": spotifyPlaylistLink(id),
		},
	}
	playlist.Tracks.Total = len(entity.TrackList)
	playlist.Items.Total = len(entity.TrackList)
	return &playlist, nil
}

func (s *Spotify) fetchPlaylistTracksFromEmbed(id string) ([]model.Song, error) {
	return s.fetchPlaylistTracksFromEmbedContext(context.Background(), id)
}

func (s *Spotify) fetchPlaylistTracksFromEmbedContext(
	ctx context.Context,
	id string,
) ([]model.Song, error) {
	entity, err := s.fetchPlaylistEmbedEntityContext(ctx, id)
	if err != nil {
		return nil, err
	}
	return spotifySongsFromEmbed(*entity), nil
}

func (s *Spotify) fetchPlaylistEmbedEntity(id string) (*spotifyEmbedEntity, error) {
	return s.fetchPlaylistEmbedEntityContext(context.Background(), id)
}

func (s *Spotify) fetchPlaylistEmbedEntityContext(
	parent context.Context,
	id string,
) (*spotifyEmbedEntity, error) {
	return s.fetchSpotifyEmbedEntityContext(parent, "playlist", id)
}

type spotifyEmbedEndpoint struct {
	name string
	url  string
}

func spotifyEmbedEndpoints(entityType string, id string) []spotifyEmbedEndpoint {
	entityType = strings.TrimSpace(entityType)
	return []spotifyEmbedEndpoint{
		{
			name: "open.spotify.com",
			url: strings.TrimRight(spotifyOpenBase, "/") +
				"/embed/" + url.PathEscape(entityType) + "/" + url.PathEscape(id),
		},
		{
			name: "embed.spotify.com",
			url: strings.TrimRight(spotifyEmbedBase, "/") +
				"/?uri=" + url.QueryEscape("spotify:"+entityType+":"+id),
		},
	}
}

func (s *Spotify) fetchSpotifyEmbedEntityContext(
	parent context.Context,
	entityType string,
	id string,
) (*spotifyEmbedEntity, error) {
	return s.fetchSpotifyEmbedEntityContextWithTimeout(
		parent,
		entityType,
		id,
		defaultRequestTimeout,
	)
}

func (s *Spotify) fetchSpotifyEmbedEntityContextWithTimeout(
	parent context.Context,
	entityType string,
	id string,
	timeout time.Duration,
) (*spotifyEmbedEntity, error) {
	type embedResult struct {
		endpoint spotifyEmbedEndpoint
		entity   *spotifyEmbedEntity
		err      error
	}

	endpoints := spotifyEmbedEndpoints(entityType, id)
	results := make(chan embedResult, len(endpoints))
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	fetch := func(endpoint spotifyEmbedEndpoint) {
		entity, err := s.fetchSpotifyEmbedEndpoint(
			ctx,
			endpoint.url,
			entityType,
			id,
		)
		results <- embedResult{endpoint: endpoint, entity: entity, err: err}
	}
	for _, endpoint := range endpoints {
		go fetch(endpoint)
	}

	failures := make([]error, 0, len(endpoints))
	for range endpoints {
		select {
		case result := <-results:
			if result.err == nil {
				cancel()
				return result.entity, nil
			}
			failures = append(
				failures,
				fmt.Errorf("%s: %w", result.endpoint.name, result.err),
			)
		case <-ctx.Done():
			failures = append(failures, ctx.Err())
			return nil, fmt.Errorf(
				"spotify %s embed request: %w",
				entityType,
				errors.Join(failures...),
			)
		}
	}
	return nil, fmt.Errorf(
		"spotify %s embed request: %w",
		entityType,
		errors.Join(failures...),
	)
}

func (s *Spotify) fetchPlaylistTracksFromPublicAlternatives(
	id string,
) ([]model.Song, error) {
	type fallbackResult struct {
		name  string
		songs []model.Song
		err   error
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()
	results := make(chan fallbackResult, 2)
	remaining := 1

	go func() {
		songs, err := s.fetchPlaylistTracksFromEmbedContext(ctx, id)
		results <- fallbackResult{name: "public embed", songs: songs, err: err}
	}()
	if _, ok := spotifyChartCountryForPlaylist(id); ok {
		remaining++
		go func() {
			songs, err := s.fetchTopListTracksFromChart(ctx, id)
			results <- fallbackResult{name: "spotify chart", songs: songs, err: err}
		}()
	}

	failures := make([]error, 0, remaining)
	for range remaining {
		select {
		case result := <-results:
			if result.err == nil {
				cancel()
				return result.songs, nil
			}
			failures = append(
				failures,
				fmt.Errorf("%s: %w", result.name, result.err),
			)
		case <-ctx.Done():
			failures = append(failures, ctx.Err())
			return nil, errors.Join(failures...)
		}
	}
	return nil, errors.Join(failures...)
}

func (s *Spotify) fetchSpotifyEmbedEndpoint(
	ctx context.Context,
	endpoint string,
	entityType string,
	id string,
) (*spotifyEmbedEntity, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		endpoint,
		nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "go-music-dl-spotify/1.0")

	resp, err := spotifyHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("spotify embed request: %w", err)
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxAPIResponseSize))
	closeErr := resp.Body.Close()
	if readErr != nil {
		return nil, fmt.Errorf("spotify embed response: %w", readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("spotify embed response close: %w", closeErr)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf(
			"spotify embed request failed with status %d",
			resp.StatusCode,
		)
	}

	payload, err := spotifyNextDataPayload(string(body))
	if err != nil {
		return nil, err
	}
	var page spotifyEmbedPageResponse
	if err := json.Unmarshal([]byte(payload), &page); err != nil {
		return nil, fmt.Errorf("spotify embed json: %w", err)
	}
	if page.Props.PageProps.Status == http.StatusUnavailableForLegalReasons {
		return nil, fmt.Errorf(
			"spotify %s unavailable in the current region (HTTP 451)",
			entityType,
		)
	}
	entity := page.Props.PageProps.State.Data.Entity
	if !strings.EqualFold(strings.TrimSpace(entity.Type), entityType) {
		return nil, fmt.Errorf("spotify embed returned entity type %q", entity.Type)
	}
	if firstSpotifyNonEmpty(entity.ID, id) == "" {
		return nil, fmt.Errorf(
			"spotify embed response did not contain a %s",
			entityType,
		)
	}
	return &entity, nil
}

func spotifyNextDataPayload(document string) (string, error) {
	const marker = `id="__NEXT_DATA__"`
	markerIndex := strings.Index(document, marker)
	if markerIndex < 0 {
		return "", errors.New("spotify embed page did not contain __NEXT_DATA__")
	}
	scriptStart := strings.LastIndex(document[:markerIndex], "<script")
	if scriptStart < 0 {
		return "", errors.New("spotify embed __NEXT_DATA__ script was malformed")
	}
	tagEndOffset := strings.Index(document[markerIndex:], ">")
	if tagEndOffset < 0 {
		return "", errors.New("spotify embed __NEXT_DATA__ script tag was malformed")
	}
	payloadStart := markerIndex + tagEndOffset + 1
	payloadEndOffset := strings.Index(document[payloadStart:], "</script>")
	if payloadEndOffset < 0 {
		return "", errors.New("spotify embed __NEXT_DATA__ payload was incomplete")
	}
	payload := strings.TrimSpace(document[payloadStart : payloadStart+payloadEndOffset])
	if payload == "" {
		return "", errors.New("spotify embed __NEXT_DATA__ payload was empty")
	}
	return payload, nil
}

func spotifySongsFromEmbed(entity spotifyEmbedEntity) []model.Song {
	cover := spotifyCover(entity.CoverArt.Sources)
	songs := make([]model.Song, 0, len(entity.TrackList))
	for _, item := range entity.TrackList {
		id := strings.TrimPrefix(strings.TrimSpace(item.URI), "spotify:track:")
		name := cleanSpotifyEmbedText(item.Title)
		if id == "" || id == strings.TrimSpace(item.URI) || name == "" {
			continue
		}
		previewURL := ""
		if item.AudioPreview != nil {
			previewURL = strings.TrimSpace(item.AudioPreview.URL)
		}
		extra := map[string]string{
			"type":            "track",
			"spotify_id":      id,
			"preview_only":    "true",
			"metadata_source": "embed",
		}
		if item.IsExplicit {
			extra["explicit"] = "true"
		}
		if !item.IsPlayable {
			extra["playable"] = "false"
		}
		songs = append(songs, model.Song{
			Source:   "spotify",
			ID:       id,
			Name:     name,
			Artist:   cleanSpotifyEmbedText(item.Subtitle),
			Duration: item.Duration / 1000,
			Cover:    cover,
			URL:      previewURL,
			Ext:      "mp3",
			Link:     spotifyTrackLink(id),
			Extra:    extra,
		})
	}
	return songs
}

func spotifySongFromEmbed(entity spotifyEmbedEntity) (model.Song, error) {
	id := strings.TrimPrefix(strings.TrimSpace(entity.URI), "spotify:track:")
	if !strings.EqualFold(strings.TrimSpace(entity.Type), "track") {
		return model.Song{}, fmt.Errorf(
			"spotify public embed returned %q instead of a track",
			entity.Type,
		)
	}
	if strings.TrimSpace(entity.ID) != "" {
		id = strings.TrimSpace(entity.ID)
	}
	if id == "" || id == strings.TrimSpace(entity.URI) {
		return model.Song{}, errors.New(
			"spotify public embed did not contain a track id",
		)
	}

	name := firstSpotifyNonEmpty(
		cleanSpotifyEmbedText(entity.Name),
		cleanSpotifyEmbedText(entity.Title),
	)
	if name == "" {
		return model.Song{}, errors.New(
			"spotify public embed did not contain a track name",
		)
	}
	artist := spotifyArtists(entity.Artists)
	if artist == "" {
		artist = cleanSpotifyEmbedText(entity.Subtitle)
	}
	previewURL := ""
	if entity.AudioPreview != nil {
		previewURL = strings.TrimSpace(entity.AudioPreview.URL)
	}
	cover := spotifyCover(entity.VisualIdentity.Image)
	if cover == "" {
		cover = spotifyCover(entity.CoverArt.Sources)
	}

	return model.Song{
		Source:   "spotify",
		ID:       id,
		Name:     name,
		Artist:   artist,
		Duration: entity.Duration / 1000,
		Cover:    cover,
		URL:      previewURL,
		Ext:      "mp3",
		Link:     spotifyTrackLink(id),
		Extra: map[string]string{
			"type":            "track",
			"spotify_id":      id,
			"preview_only":    "true",
			"metadata_source": "embed",
		},
	}, nil
}

func cleanSpotifyEmbedText(value string) string {
	value = strings.ReplaceAll(value, "\u00a0", " ")
	return strings.Join(strings.Fields(value), " ")
}
