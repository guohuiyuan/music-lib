package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
)

const spotifyChartTrackLimit = 50

var (
	spotifyKworbBase = "https://kworb.net"

	spotifyKworbRowPattern = regexp.MustCompile(
		`(?is)<tr>\s*<td class="np">(\d+)</td>.*?</tr>`,
	)
	spotifyKworbTrackPattern = regexp.MustCompile(
		`(?is)<a[^>]+href="(?:\.\./)?track/([A-Za-z0-9_-]+)\.html"[^>]*>(.*?)</a>`,
	)
	spotifyKworbArtistPattern = regexp.MustCompile(
		`(?is)<a[^>]+href="(?:\.\./)?artist/[^"]+\.html"[^>]*>(.*?)</a>`,
	)
	spotifyKworbDatePattern = regexp.MustCompile(
		`(?is)<span class="pagetitle">[^<]*?(\d{4}/\d{2}/\d{2})`,
	)
	spotifyHTMLTagPattern = regexp.MustCompile(`(?is)<[^>]+>`)
)

type spotifyOEmbedResponse struct {
	Title        string `json:"title"`
	ThumbnailURL string `json:"thumbnail_url"`
}

type spotifyKworbEntry struct {
	Rank   int
	ID     string
	Title  string
	Artist string
}

func spotifyChartCountryForPlaylist(playlistID string) (string, bool) {
	playlistID = strings.TrimSpace(playlistID)
	for _, definition := range spotifyTopListDefinitions {
		if playlistID == definition.PlaylistID &&
			strings.TrimSpace(definition.ChartCountry) != "" {
			return definition.ChartCountry, true
		}
	}
	return "", false
}

func (s *Spotify) fetchTopListTracksFromChart(
	ctx context.Context,
	playlistID string,
) ([]model.Song, error) {
	country, ok := spotifyChartCountryForPlaylist(playlistID)
	if !ok {
		return nil, errors.New("spotify playlist has no public chart fallback")
	}

	endpoint := strings.TrimRight(spotifyKworbBase, "/") +
		"/spotify/country/" + url.PathEscape(country) + "_daily.html"
	status, body, err := spotifyPublicGetContextAccept(
		ctx,
		endpoint,
		nil,
		"text/html,application/xhtml+xml",
	)
	if err != nil {
		return nil, fmt.Errorf("spotify chart fallback request: %w", err)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("spotify chart fallback returned status %d", status)
	}

	entries, chartDate, err := parseSpotifyKworbChart(string(body))
	if err != nil {
		return nil, err
	}
	if len(entries) > spotifyChartTrackLimit {
		entries = entries[:spotifyChartTrackLimit]
	}

	cover := ""
	metadata, err := s.fetchPlaylistFromOEmbedContext(ctx, playlistID)
	if err == nil {
		cover = spotifyCover(metadata.Images)
	}

	songs := make([]model.Song, 0, len(entries))
	for _, entry := range entries {
		extra := map[string]string{
			"type":            "track",
			"spotify_id":      entry.ID,
			"metadata_source": "kworb",
			"chart_source":    "spotify_daily",
			"chart_country":   country,
			"rank":            strconv.Itoa(entry.Rank),
		}
		if chartDate != "" {
			extra["chart_date"] = chartDate
		}
		songs = append(songs, model.Song{
			Source: "spotify",
			ID:     entry.ID,
			Name:   entry.Title,
			Artist: entry.Artist,
			Cover:  cover,
			Link:   spotifyTrackLink(entry.ID),
			Extra:  extra,
		})
	}
	if len(songs) == 0 {
		return nil, errors.New("spotify chart fallback did not contain any tracks")
	}
	return songs, nil
}

func parseSpotifyKworbChart(document string) ([]spotifyKworbEntry, string, error) {
	rows := spotifyKworbRowPattern.FindAllStringSubmatch(document, -1)
	if len(rows) == 0 {
		return nil, "", errors.New("spotify chart fallback page did not contain chart rows")
	}

	entries := make([]spotifyKworbEntry, 0, len(rows))
	for _, row := range rows {
		rank, err := strconv.Atoi(strings.TrimSpace(row[1]))
		if err != nil {
			continue
		}
		track := spotifyKworbTrackPattern.FindStringSubmatch(row[0])
		if len(track) < 3 {
			continue
		}
		title := cleanSpotifyChartText(track[2])
		if title == "" {
			continue
		}

		artistMatches := spotifyKworbArtistPattern.FindAllStringSubmatch(row[0], -1)
		artists := make([]string, 0, len(artistMatches))
		seenArtists := make(map[string]struct{}, len(artistMatches))
		for _, artistMatch := range artistMatches {
			artist := cleanSpotifyChartText(artistMatch[1])
			if artist == "" {
				continue
			}
			if _, exists := seenArtists[artist]; exists {
				continue
			}
			seenArtists[artist] = struct{}{}
			artists = append(artists, artist)
		}
		entries = append(entries, spotifyKworbEntry{
			Rank:   rank,
			ID:     strings.TrimSpace(track[1]),
			Title:  title,
			Artist: strings.Join(artists, ", "),
		})
	}
	if len(entries) == 0 {
		return nil, "", errors.New("spotify chart fallback rows did not contain tracks")
	}

	chartDate := ""
	if date := spotifyKworbDatePattern.FindStringSubmatch(document); len(date) >= 2 {
		chartDate = strings.TrimSpace(date[1])
	}
	return entries, chartDate, nil
}

func cleanSpotifyChartText(value string) string {
	value = spotifyHTMLTagPattern.ReplaceAllString(value, " ")
	value = html.UnescapeString(value)
	return strings.Join(strings.Fields(value), " ")
}

func (s *Spotify) fetchPlaylistFromOEmbed(id string) (*spotifyPlaylist, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()
	return s.fetchPlaylistFromOEmbedContext(ctx, id)
}

func (s *Spotify) fetchPlaylistFromOEmbedContext(
	ctx context.Context,
	id string,
) (*spotifyPlaylist, error) {
	type oEmbedResult struct {
		playlist *spotifyPlaylist
		err      error
	}

	query := url.Values{}
	query.Set("url", "spotify:playlist:"+id)
	endpoints := []string{
		strings.TrimRight(spotifyEmbedBase, "/") + "/oembed",
		strings.TrimRight(spotifyOpenBase, "/") + "/oembed",
	}

	requestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan oEmbedResult, len(endpoints))
	for _, endpoint := range endpoints {
		go func(endpoint string) {
			playlist, err := fetchSpotifyOEmbedEndpoint(requestCtx, endpoint, query, id)
			results <- oEmbedResult{playlist: playlist, err: err}
		}(endpoint)
	}

	failures := make([]error, 0, len(endpoints))
	for range endpoints {
		select {
		case result := <-results:
			if result.err == nil {
				cancel()
				return result.playlist, nil
			}
			failures = append(failures, result.err)
		case <-requestCtx.Done():
			failures = append(failures, requestCtx.Err())
			return nil, fmt.Errorf("spotify oembed request: %w", errors.Join(failures...))
		}
	}
	return nil, fmt.Errorf("spotify oembed request: %w", errors.Join(failures...))
}

func fetchSpotifyOEmbedEndpoint(
	ctx context.Context,
	endpoint string,
	query url.Values,
	id string,
) (*spotifyPlaylist, error) {
	status, body, err := spotifyPublicGetContextAccept(
		ctx,
		endpoint,
		query,
		"application/json",
	)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("spotify oembed returned status %d", status)
	}

	var response spotifyOEmbedResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("spotify oembed json: %w", err)
	}
	title := strings.TrimSpace(response.Title)
	if title == "" {
		return nil, errors.New("spotify oembed response title is empty")
	}

	return &spotifyPlaylist{
		ID:          id,
		Name:        title,
		Description: "Spotify public playlist",
		Images: []spotifyImage{
			{URL: strings.TrimSpace(response.ThumbnailURL)},
		},
		Owner: spotifyPlaylistOwner{DisplayName: "Spotify"},
		ExternalURLs: map[string]string{
			"spotify": spotifyPlaylistLink(id),
		},
	}, nil
}
