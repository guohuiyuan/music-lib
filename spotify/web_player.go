package spotify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/guohuiyuan/music-lib/model"
)

const (
	spotifyWebPlayerUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36"
	spotifyWebSearchOperation = "searchDesktop"
	spotifyWebSearchHash      = "fcad5a3e0d5af727fb76966f06971c19cfa2275e6ff7671196753e008611873c"
	spotifyWebPlayerTokenTTL  = 45 * time.Minute
	spotifyWebSearchCacheTTL  = time.Minute
	spotifyWebSearchCacheSize = 64
)

var (
	spotifyClientTokenBase     = "https://clienttoken.spotify.com"
	spotifyPartnerBase         = "https://api-partner.spotify.com"
	spotifyWebPlayerHTTPClient = newSpotifyHTTPClient(30 * time.Second)
)

type spotifyTOTPSecret struct {
	version int
	values  []byte
}

// Spotify rotates the Web Player TOTP secret occasionally. Try the newest
// known versions in order so a server-side rollout does not immediately break
// the anonymous fallback.
var spotifyWebPlayerTOTPSecrets = []spotifyTOTPSecret{
	{
		version: 61,
		values: []byte{
			44, 55, 47, 42, 70, 40, 34, 114, 76, 74, 50, 111, 120, 97,
			75, 76, 94, 102, 43, 69, 49, 120, 118, 80, 64, 78,
		},
	},
	{
		version: 60,
		values: []byte{
			79, 109, 69, 123, 90, 65, 46, 74, 94, 34, 58, 48, 70, 71,
			92, 85, 122, 63, 91, 64, 87, 87,
		},
	},
	{
		version: 59,
		values: []byte{
			123, 105, 79, 70, 110, 59, 52, 125, 60, 49, 80, 70, 89, 75,
			80, 86, 63, 53, 123, 37, 117, 49, 52, 93, 77, 62, 47, 86,
			48, 104, 68, 72,
		},
	},
}

type spotifyWebPlayerAuth struct {
	cacheKey      string
	accessToken   string
	clientToken   string
	clientVersion string
	deviceID      string
	cookie        string
	expiresAt     time.Time
}

var spotifyWebPlayerAuthCache struct {
	sync.Mutex
	value *spotifyWebPlayerAuth
}

type spotifyWebSearchCacheEntry struct {
	response  *spotifyWebSearchResponse
	expiresAt time.Time
}

var spotifyWebPlayerSearchCache struct {
	sync.Mutex
	entries map[string]spotifyWebSearchCacheEntry
}

type spotifyWebPlayerTokenResponse struct {
	AccessToken string `json:"accessToken"`
	ClientID    string `json:"clientId"`
}

type spotifyWebClientTokenResponse struct {
	GrantedToken struct {
		Token string `json:"token"`
	} `json:"granted_token"`
}

type spotifyWebSearchResponse struct {
	Data struct {
		SearchV2 struct {
			TracksV2    spotifyWebSearchConnection[spotifyWebSearchItem[spotifyWebTrack]]    `json:"tracksV2"`
			Tracks      spotifyWebSearchConnection[spotifyWebSearchItem[spotifyWebTrack]]    `json:"tracks"`
			AlbumsV2    spotifyWebSearchConnection[spotifyWebSearchItem[spotifyWebAlbum]]    `json:"albumsV2"`
			Albums      spotifyWebSearchConnection[spotifyWebSearchItem[spotifyWebAlbum]]    `json:"albums"`
			PlaylistsV2 spotifyWebSearchConnection[spotifyWebSearchItem[spotifyWebPlaylist]] `json:"playlistsV2"`
			Playlists   spotifyWebSearchConnection[spotifyWebSearchItem[spotifyWebPlaylist]] `json:"playlists"`
		} `json:"searchV2"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type spotifyWebSearchConnection[T any] struct {
	Items      []T `json:"items"`
	TotalCount int `json:"totalCount"`
}

type spotifyWebSearchData[T any] struct {
	Data T `json:"data"`
}

type spotifyWebSearchItem[T any] struct {
	Item   *spotifyWebSearchData[T] `json:"item"`
	ItemV2 *spotifyWebSearchData[T] `json:"itemV2"`
	Data   *T                       `json:"data"`
}

func (item spotifyWebSearchItem[T]) value() *T {
	switch {
	case item.Item != nil:
		return &item.Item.Data
	case item.ItemV2 != nil:
		return &item.ItemV2.Data
	default:
		return item.Data
	}
}

type spotifyWebArtistConnection struct {
	Items []spotifyWebArtist `json:"items"`
}

type spotifyWebArtist struct {
	URI     string `json:"uri"`
	Profile struct {
		Name string `json:"name"`
	} `json:"profile"`
}

type spotifyWebCoverArt struct {
	Sources []spotifyImage `json:"sources"`
}

type spotifyWebAlbumReference struct {
	URI      string             `json:"uri"`
	Name     string             `json:"name"`
	CoverArt spotifyWebCoverArt `json:"coverArt"`
}

type spotifyWebTrack struct {
	URI           string                      `json:"uri"`
	Name          string                      `json:"name"`
	Duration      spotifyWebDuration          `json:"duration"`
	TrackNumber   int                         `json:"trackNumber"`
	DiscNumber    int                         `json:"discNumber"`
	Artists       spotifyWebArtistConnection  `json:"artists"`
	AlbumOfTrack  spotifyWebAlbumReference    `json:"albumOfTrack"`
	Previews      spotifyWebPreviewConnection `json:"previews"`
	Playability   spotifyWebPlayability       `json:"playability"`
	ContentRating spotifyWebContentRating     `json:"contentRating"`
}

type spotifyWebDuration struct {
	TotalMilliseconds int `json:"totalMilliseconds"`
}

type spotifyWebPreviewConnection struct {
	AudioPreviews spotifyWebPreviewItems `json:"audioPreviews"`
}

type spotifyWebPreviewItems struct {
	Items []struct {
		URL string `json:"url"`
	} `json:"items"`
}

type spotifyWebPlayability struct {
	Playable *bool `json:"playable"`
}

type spotifyWebContentRating struct {
	Label string `json:"label"`
}

type spotifyWebAlbum struct {
	URI      string                     `json:"uri"`
	Name     string                     `json:"name"`
	Artists  spotifyWebArtistConnection `json:"artists"`
	CoverArt spotifyWebCoverArt         `json:"coverArt"`
	Date     struct {
		Year      int    `json:"year"`
		ISOString string `json:"isoString"`
	} `json:"date"`
	Tracks struct {
		TotalCount int `json:"totalCount"`
	} `json:"tracks"`
}

type spotifyWebPlaylist struct {
	URI         string                   `json:"uri"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	OwnerV2     *spotifyWebPlaylistOwner `json:"ownerV2"`
	Owner       *spotifyWebPlaylistOwner `json:"owner"`
	Images      spotifyWebPlaylistImages `json:"images"`
	Content     struct {
		TotalCount int `json:"totalCount"`
	} `json:"content"`
}

type spotifyWebPlaylistOwner struct {
	Data struct {
		Name string `json:"name"`
		URI  string `json:"uri"`
	} `json:"data"`
	Name string `json:"name"`
	URI  string `json:"uri"`
}

type spotifyWebPlaylistImages struct {
	Items []struct {
		Sources []spotifyImage `json:"sources"`
	} `json:"items"`
}

func (s *Spotify) searchSongsFromWebPlayer(
	ctx context.Context,
	keyword string,
	limit int,
) ([]model.Song, error) {
	response, err := s.searchSpotifyWebPlayer(ctx, keyword, 0)
	if err != nil {
		return nil, err
	}
	items := response.Data.SearchV2.TracksV2.Items
	if len(items) == 0 {
		items = response.Data.SearchV2.Tracks.Items
	}
	songs := make([]model.Song, 0, len(items))
	for _, item := range items {
		track := item.value()
		if track == nil {
			continue
		}
		song, ok := spotifySongFromWebPlayerTrack(*track)
		if ok {
			songs = append(songs, song)
			if limit > 0 && len(songs) >= limit {
				break
			}
		}
	}
	return songs, nil
}

func (s *Spotify) searchAlbumsFromWebPlayer(
	ctx context.Context,
	keyword string,
	limit int,
) ([]model.Playlist, error) {
	response, err := s.searchSpotifyWebPlayer(ctx, keyword, 0)
	if err != nil {
		return nil, err
	}
	items := response.Data.SearchV2.AlbumsV2.Items
	if len(items) == 0 {
		items = response.Data.SearchV2.Albums.Items
	}
	albums := make([]model.Playlist, 0, len(items))
	for _, item := range items {
		album := item.value()
		if album == nil {
			continue
		}
		result, ok := spotifyAlbumFromWebPlayer(*album)
		if ok {
			albums = append(albums, result)
			if limit > 0 && len(albums) >= limit {
				break
			}
		}
	}
	return albums, nil
}

func (s *Spotify) searchPlaylistsFromWebPlayer(
	ctx context.Context,
	keyword string,
	limit int,
) ([]model.Playlist, error) {
	response, err := s.searchSpotifyWebPlayer(ctx, keyword, 0)
	if err != nil {
		return nil, err
	}
	items := response.Data.SearchV2.PlaylistsV2.Items
	if len(items) == 0 {
		items = response.Data.SearchV2.Playlists.Items
	}
	playlists := make([]model.Playlist, 0, len(items))
	for _, item := range items {
		playlist := item.value()
		if playlist == nil {
			continue
		}
		result, ok := spotifyPlaylistFromWebPlayer(*playlist)
		if ok {
			playlists = append(playlists, result)
			if limit > 0 && len(playlists) >= limit {
				break
			}
		}
	}
	return playlists, nil
}

func (s *Spotify) searchSpotifyWebPlayer(
	ctx context.Context,
	keyword string,
	offset int,
) (*spotifyWebSearchResponse, error) {
	if offset < 0 {
		offset = 0
	}
	searchCacheKey := spotifyWebPlayerSearchCacheKey(keyword, offset)
	if response := spotifyLoadWebSearchCache(searchCacheKey); response != nil {
		return response, nil
	}

	payload := map[string]any{
		"operationName": spotifyWebSearchOperation,
		"variables": map[string]any{
			"searchTerm":                    keyword,
			"offset":                        offset,
			"limit":                         spotifySearchLimit,
			"numberOfTopResults":            5,
			"includeAudiobooks":             true,
			"includeArtistHasConcertsField": false,
			"includePreReleases":            true,
			"includeAuthors":                false,
		},
		"extensions": map[string]any{
			"persistedQuery": map[string]any{
				"version":    1,
				"sha256Hash": spotifyWebSearchHash,
			},
		},
	}

	authRetried := false
	for {
		auth, err := s.spotifyAnonymousAuth(ctx)
		if err != nil {
			return nil, err
		}

		body, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("spotify web player search payload: %w", err)
		}
		endpoint := strings.TrimRight(spotifyPartnerBase, "/") + "/pathfinder/v2/query"
		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			endpoint,
			bytes.NewReader(body),
		)
		if err != nil {
			return nil, err
		}
		spotifySetWebPlayerHeaders(req, auth)
		req.Header.Set("Content-Type", "application/json;charset=UTF-8")

		resp, err := spotifyDoWebPlayerRequest(req)
		if err != nil {
			return nil, fmt.Errorf("spotify web player search request: %w", err)
		}
		responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxAPIResponseSize))
		closeErr := resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("spotify web player search response: %w", readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("spotify web player search response close: %w", closeErr)
		}

		if (resp.StatusCode == http.StatusUnauthorized ||
			resp.StatusCode == http.StatusForbidden) &&
			!authRetried {
			authRetried = true
			spotifyInvalidateWebPlayerAuth(auth)
			continue
		}
		if resp.StatusCode < http.StatusOK ||
			resp.StatusCode >= http.StatusMultipleChoices {
			return nil, fmt.Errorf(
				"spotify web player search: status %d: %s",
				resp.StatusCode,
				spotifyResponseMessage(responseBody),
			)
		}

		var result spotifyWebSearchResponse
		if err := json.Unmarshal(responseBody, &result); err != nil {
			return nil, fmt.Errorf("spotify web player search json: %w", err)
		}
		if message := firstSpotifyGraphQLError(result.Errors); message != "" &&
			spotifyWebSearchResultCount(result) == 0 {
			return nil, fmt.Errorf("spotify web player search: %s", message)
		}
		spotifyStoreWebSearchCache(searchCacheKey, &result)
		return &result, nil
	}
}

func (s *Spotify) spotifyAnonymousAuth(
	ctx context.Context,
) (*spotifyWebPlayerAuth, error) {
	cacheKey := spotifyWebPlayerCacheKey()
	spotifyWebPlayerAuthCache.Lock()
	defer spotifyWebPlayerAuthCache.Unlock()

	if cached := spotifyWebPlayerAuthCache.value; cached != nil &&
		cached.cacheKey == cacheKey &&
		time.Now().Add(2*time.Minute).Before(cached.expiresAt) {
		return cached, nil
	}

	auth, err := s.fetchSpotifyWebPlayerAuth(ctx, cacheKey)
	if err != nil {
		return nil, fmt.Errorf("spotify anonymous web player: %w", err)
	}
	spotifyWebPlayerAuthCache.value = auth
	return auth, nil
}

func (s *Spotify) fetchSpotifyWebPlayerAuth(
	ctx context.Context,
	cacheKey string,
) (*spotifyWebPlayerAuth, error) {
	deviceID, cookie, clientVersion, err := s.fetchSpotifyWebPlayerBootstrap(ctx)
	if err != nil {
		return nil, err
	}

	var failures []error
	for _, secret := range spotifyWebPlayerTOTPSecrets {
		totp := spotifyGenerateTOTP(secret.values, time.Now())
		token, err := s.fetchSpotifyWebPlayerToken(ctx, cookie, secret.version, totp)
		if err != nil {
			failures = append(
				failures,
				fmt.Errorf("TOTP v%d: %w", secret.version, err),
			)
			continue
		}
		clientToken, err := s.fetchSpotifyWebPlayerClientToken(
			ctx,
			cookie,
			clientVersion,
			token.ClientID,
			deviceID,
		)
		if err != nil {
			return nil, err
		}
		return &spotifyWebPlayerAuth{
			cacheKey:      cacheKey,
			accessToken:   token.AccessToken,
			clientToken:   clientToken,
			clientVersion: clientVersion,
			deviceID:      deviceID,
			cookie:        cookie,
			expiresAt:     time.Now().Add(spotifyWebPlayerTokenTTL),
		}, nil
	}
	return nil, fmt.Errorf(
		"unable to obtain an anonymous Spotify token; the Web Player protocol may have rotated: %w",
		errors.Join(failures...),
	)
}

func (s *Spotify) fetchSpotifyWebPlayerBootstrap(
	ctx context.Context,
) (deviceID string, cookie string, clientVersion string, err error) {
	endpoint := strings.TrimRight(spotifyOpenBase, "/") + "/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", spotifyWebPlayerUserAgent)

	resp, err := spotifyDoWebPlayerRequest(req)
	if err != nil {
		return "", "", "", fmt.Errorf(
			"spotify web player bootstrap (check network access or HTTP(S)_PROXY; Windows system proxy is detected automatically): %w",
			err,
		)
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxAPIResponseSize))
	closeErr := resp.Body.Close()
	if readErr != nil {
		return "", "", "", fmt.Errorf("spotify web player bootstrap response: %w", readErr)
	}
	if closeErr != nil {
		return "", "", "", fmt.Errorf("spotify web player bootstrap close: %w", closeErr)
	}
	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		return "", "", "", fmt.Errorf(
			"spotify web player bootstrap: status %d: %s",
			resp.StatusCode,
			spotifyResponseMessage(body),
		)
	}

	for _, item := range resp.Cookies() {
		if item.Name == "sp_t" {
			deviceID = strings.TrimSpace(item.Value)
			break
		}
	}
	if deviceID == "" {
		deviceID, err = spotifyRandomUUID()
		if err != nil {
			return "", "", "", err
		}
	}
	cookie = "sp_t=" + deviceID
	clientVersion = spotifyAppServerClientVersion(string(body))
	if clientVersion == "" {
		return "", "", "", errors.New(
			"spotify web player bootstrap did not contain a client version",
		)
	}
	return deviceID, cookie, clientVersion, nil
}

func (s *Spotify) fetchSpotifyWebPlayerToken(
	ctx context.Context,
	cookie string,
	totpVersion int,
	totp string,
) (*spotifyWebPlayerTokenResponse, error) {
	query := url.Values{}
	query.Set("reason", "init")
	query.Set("productType", "web-player")
	query.Set("totp", totp)
	query.Set("totpVer", strconv.Itoa(totpVersion))
	query.Set("totpServer", totp)

	endpoint := strings.TrimRight(spotifyOpenBase, "/") + "/api/token?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	spotifySetAnonymousCommonHeaders(req)
	req.Header.Set("Cookie", cookie)

	var result spotifyWebPlayerTokenResponse
	if err := spotifyDoWebPlayerJSON(req, &result); err != nil {
		return nil, err
	}
	if strings.TrimSpace(result.AccessToken) == "" ||
		strings.TrimSpace(result.ClientID) == "" {
		return nil, errors.New("spotify Web Player token response was incomplete")
	}
	return &result, nil
}

func (s *Spotify) fetchSpotifyWebPlayerClientToken(
	ctx context.Context,
	cookie string,
	clientVersion string,
	clientID string,
	deviceID string,
) (string, error) {
	payload := map[string]any{
		"client_data": map[string]any{
			"client_version": clientVersion,
			"client_id":      clientID,
			"js_sdk_data": map[string]any{
				"device_brand": "unknown",
				"device_model": "unknown",
				"os":           "windows",
				"os_version":   "NT 10.0",
				"device_id":    deviceID,
				"device_type":  "computer",
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	endpoint := strings.TrimRight(spotifyClientTokenBase, "/") + "/v1/clienttoken"
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return "", err
	}
	spotifySetAnonymousCommonHeaders(req)
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	var result spotifyWebClientTokenResponse
	if err := spotifyDoWebPlayerJSON(req, &result); err != nil {
		return "", fmt.Errorf("spotify client token: %w", err)
	}
	token := strings.TrimSpace(result.GrantedToken.Token)
	if token == "" {
		return "", errors.New("spotify client token response was empty")
	}
	return token, nil
}

func spotifyDoWebPlayerJSON(req *http.Request, target any) error {
	resp, err := spotifyDoWebPlayerRequest(req)
	if err != nil {
		return err
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxAPIResponseSize))
	closeErr := resp.Body.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf(
			"status %d: %s",
			resp.StatusCode,
			spotifyResponseMessage(body),
		)
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("json: %w", err)
	}
	return nil
}

func spotifyDoWebPlayerRequest(req *http.Request) (*http.Response, error) {
	const attempts = 2
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		current := req.Clone(req.Context())
		if req.Body != nil {
			if req.GetBody == nil {
				return nil, errors.New(
					"spotify web player request body cannot be replayed",
				)
			}
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			current.Body = body
		}
		resp, err := spotifyWebPlayerHTTPClient.Do(current)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if req.Context().Err() != nil || attempt == attempts-1 {
			break
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 250 * time.Millisecond)
		select {
		case <-req.Context().Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil, req.Context().Err()
		case <-timer.C:
		}
	}
	return nil, lastErr
}

func spotifySetAnonymousCommonHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Origin", strings.TrimRight(spotifyOpenBase, "/"))
	req.Header.Set("Referer", strings.TrimRight(spotifyOpenBase, "/")+"/")
	req.Header.Set("User-Agent", spotifyWebPlayerUserAgent)
}

func spotifySetWebPlayerHeaders(req *http.Request, auth *spotifyWebPlayerAuth) {
	spotifySetAnonymousCommonHeaders(req)
	req.Header.Set("Authorization", "Bearer "+auth.accessToken)
	req.Header.Set("Client-Token", auth.clientToken)
	req.Header.Set("Spotify-App-Version", auth.clientVersion)
	req.Header.Set("Cookie", auth.cookie)
}

func spotifyGenerateTOTP(secret []byte, at time.Time) string {
	transformed := make([]byte, len(secret))
	for index, value := range secret {
		transformed[index] = value ^ byte(index%33+9)
	}
	var keyBuilder strings.Builder
	for _, value := range transformed {
		keyBuilder.WriteString(strconv.Itoa(int(value)))
	}

	counter := uint64(at.Unix() / 30)
	var message [8]byte
	binary.BigEndian.PutUint64(message[:], counter)
	mac := hmac.New(sha1.New, []byte(keyBuilder.String()))
	_, _ = mac.Write(message[:])
	digest := mac.Sum(nil)
	offset := digest[len(digest)-1] & 0x0f
	value := binary.BigEndian.Uint32(digest[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", value%1_000_000)
}

func spotifyAppServerClientVersion(document string) string {
	const marker = `<script id="appServerConfig" type="text/plain">`
	start := strings.Index(document, marker)
	if start < 0 {
		return ""
	}
	start += len(marker)
	end := strings.Index(document[start:], "</script>")
	if end < 0 {
		return ""
	}
	encoded := strings.TrimSpace(document[start : start+end])
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return ""
	}
	var config struct {
		ClientVersion string `json:"clientVersion"`
	}
	if err := json.Unmarshal(decoded, &config); err != nil {
		return ""
	}
	return strings.TrimSpace(config.ClientVersion)
}

func spotifyRandomUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("spotify web player device id: %w", err)
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	encoded := hex.EncodeToString(value[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" +
		encoded[12:16] + "-" + encoded[16:20] + "-" +
		encoded[20:32], nil
}

func spotifyWebPlayerCacheKey() string {
	return strings.TrimRight(spotifyOpenBase, "/") + "\x00" +
		strings.TrimRight(spotifyClientTokenBase, "/") + "\x00" +
		strings.TrimRight(spotifyPartnerBase, "/")
}

func spotifyWebPlayerSearchCacheKey(keyword string, offset int) string {
	return spotifyWebPlayerCacheKey() + "\x00" +
		strconv.Itoa(offset) + "\x00" +
		strings.TrimSpace(keyword)
}

func spotifyLoadWebSearchCache(key string) *spotifyWebSearchResponse {
	spotifyWebPlayerSearchCache.Lock()
	defer spotifyWebPlayerSearchCache.Unlock()
	entry, exists := spotifyWebPlayerSearchCache.entries[key]
	if !exists {
		return nil
	}
	if time.Now().After(entry.expiresAt) {
		delete(spotifyWebPlayerSearchCache.entries, key)
		return nil
	}
	return entry.response
}

func spotifyStoreWebSearchCache(key string, response *spotifyWebSearchResponse) {
	if response == nil {
		return
	}
	spotifyWebPlayerSearchCache.Lock()
	defer spotifyWebPlayerSearchCache.Unlock()
	if spotifyWebPlayerSearchCache.entries == nil {
		spotifyWebPlayerSearchCache.entries = make(
			map[string]spotifyWebSearchCacheEntry,
		)
	}
	now := time.Now()
	if len(spotifyWebPlayerSearchCache.entries) >= spotifyWebSearchCacheSize {
		for cacheKey, entry := range spotifyWebPlayerSearchCache.entries {
			if now.After(entry.expiresAt) {
				delete(spotifyWebPlayerSearchCache.entries, cacheKey)
			}
		}
	}
	if len(spotifyWebPlayerSearchCache.entries) >= spotifyWebSearchCacheSize {
		for cacheKey := range spotifyWebPlayerSearchCache.entries {
			delete(spotifyWebPlayerSearchCache.entries, cacheKey)
			break
		}
	}
	spotifyWebPlayerSearchCache.entries[key] = spotifyWebSearchCacheEntry{
		response:  response,
		expiresAt: now.Add(spotifyWebSearchCacheTTL),
	}
}

func spotifyInvalidateWebPlayerAuth(auth *spotifyWebPlayerAuth) {
	spotifyWebPlayerAuthCache.Lock()
	defer spotifyWebPlayerAuthCache.Unlock()
	if auth != nil && spotifyWebPlayerAuthCache.value != nil &&
		spotifyWebPlayerAuthCache.value.accessToken == auth.accessToken {
		spotifyWebPlayerAuthCache.value = nil
	}
}

func spotifyAllowsAnonymousSearchFallback(err error) bool {
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
		http.StatusTooManyRequests:
		return true
	default:
		return false
	}
}

func spotifyResponseMessage(body []byte) string {
	var payload struct {
		Message string `json:"message"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &payload) == nil {
		for _, candidate := range []string{payload.Error.Message, payload.Message} {
			if message := strings.TrimSpace(candidate); message != "" {
				return message
			}
		}
	}
	message := strings.TrimSpace(string(body))
	if len(message) > 500 {
		message = message[:500]
	}
	if message == "" {
		message = http.StatusText(http.StatusBadGateway)
	}
	return message
}

func firstSpotifyGraphQLError(errors []struct {
	Message string `json:"message"`
}) string {
	for _, item := range errors {
		if message := strings.TrimSpace(item.Message); message != "" {
			return message
		}
	}
	return ""
}

func spotifyWebSearchResultCount(response spotifyWebSearchResponse) int {
	search := response.Data.SearchV2
	return len(search.TracksV2.Items) +
		len(search.Tracks.Items) +
		len(search.AlbumsV2.Items) +
		len(search.Albums.Items) +
		len(search.PlaylistsV2.Items) +
		len(search.Playlists.Items)
}

func spotifySongFromWebPlayerTrack(track spotifyWebTrack) (model.Song, bool) {
	id := spotifyIDFromURI(track.URI, "track")
	name := strings.TrimSpace(track.Name)
	if id == "" || name == "" {
		return model.Song{}, false
	}
	albumID := spotifyIDFromURI(track.AlbumOfTrack.URI, "album")
	previewURL := ""
	for _, item := range track.Previews.AudioPreviews.Items {
		if value := strings.TrimSpace(item.URL); value != "" {
			previewURL = value
			break
		}
	}
	extra := map[string]string{
		"type":            "track",
		"spotify_id":      id,
		"preview_only":    "true",
		"metadata_source": "web_player",
		"auth_source":     "anonymous_web_player",
	}
	if track.TrackNumber > 0 {
		extra["track_number"] = strconv.Itoa(track.TrackNumber)
	}
	if track.DiscNumber > 0 {
		extra["disc_number"] = strconv.Itoa(track.DiscNumber)
	}
	if strings.EqualFold(strings.TrimSpace(track.ContentRating.Label), "EXPLICIT") {
		extra["explicit"] = "true"
	}
	if track.Playability.Playable != nil && !*track.Playability.Playable {
		extra["playable"] = "false"
	}
	return model.Song{
		Source:   "spotify",
		ID:       id,
		Name:     name,
		Artist:   spotifyWebPlayerArtists(track.Artists),
		Album:    strings.TrimSpace(track.AlbumOfTrack.Name),
		AlbumID:  albumID,
		Duration: track.Duration.TotalMilliseconds / 1000,
		Cover:    spotifyCover(track.AlbumOfTrack.CoverArt.Sources),
		URL:      previewURL,
		Ext:      "mp3",
		Link:     spotifyTrackLink(id),
		Extra:    extra,
	}, true
}

func spotifyAlbumFromWebPlayer(album spotifyWebAlbum) (model.Playlist, bool) {
	id := spotifyIDFromURI(album.URI, "album")
	name := strings.TrimSpace(album.Name)
	if id == "" || name == "" {
		return model.Playlist{}, false
	}
	releaseDate := strings.TrimSpace(album.Date.ISOString)
	if releaseDate == "" && album.Date.Year > 0 {
		releaseDate = strconv.Itoa(album.Date.Year)
	}
	return model.Playlist{
		Source:      "spotify",
		ID:          id,
		Name:        name,
		Cover:       spotifyCover(album.CoverArt.Sources),
		TrackCount:  album.Tracks.TotalCount,
		Creator:     spotifyWebPlayerArtists(album.Artists),
		Description: releaseDate,
		Link:        spotifyAlbumLink(id),
		Extra: map[string]string{
			"type":            "album",
			"spotify_id":      id,
			"release_date":    releaseDate,
			"metadata_source": "web_player",
			"auth_source":     "anonymous_web_player",
		},
	}, true
}

func spotifyPlaylistFromWebPlayer(playlist spotifyWebPlaylist) (model.Playlist, bool) {
	id := spotifyIDFromURI(playlist.URI, "playlist")
	name := strings.TrimSpace(playlist.Name)
	if id == "" || name == "" {
		return model.Playlist{}, false
	}
	images := make([]spotifyImage, 0)
	if len(playlist.Images.Items) > 0 {
		images = playlist.Images.Items[0].Sources
	}
	owner := playlist.OwnerV2
	if owner == nil {
		owner = playlist.Owner
	}
	ownerName, ownerID := "", ""
	if owner != nil {
		ownerName = firstSpotifyNonEmpty(owner.Data.Name, owner.Name)
		ownerURI := firstSpotifyNonEmpty(owner.Data.URI, owner.URI)
		ownerID = strings.TrimPrefix(ownerURI, "spotify:user:")
	}
	return model.Playlist{
		Source:      "spotify",
		ID:          id,
		Name:        name,
		Cover:       spotifyCover(images),
		TrackCount:  playlist.Content.TotalCount,
		Creator:     ownerName,
		Description: strings.TrimSpace(playlist.Description),
		Link:        spotifyPlaylistLink(id),
		Extra: map[string]string{
			"type":            "playlist",
			"spotify_id":      id,
			"owner_id":        ownerID,
			"metadata_source": "web_player",
			"auth_source":     "anonymous_web_player",
		},
	}, true
}

func spotifyWebPlayerArtists(artists spotifyWebArtistConnection) string {
	names := make([]string, 0, len(artists.Items))
	seen := make(map[string]struct{}, len(artists.Items))
	for _, artist := range artists.Items {
		name := strings.TrimSpace(artist.Profile.Name)
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

func spotifyIDFromURI(uri string, entityType string) string {
	id, err := extractSpotifyID(uri, entityType)
	if err != nil {
		return ""
	}
	return id
}
