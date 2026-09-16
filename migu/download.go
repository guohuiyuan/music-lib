package migu

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
)

const (
	miguListenURL      = "https://c.musicapp.migu.cn/MIGUM2.0/v2.1/content/listen-url"
	miguAndroidAppKey  = "308202333082019c"
	miguAndroidUA      = "Android_migu/7.41.13 okhttp/3.12.13"
	miguAndroidChannel = "0146832"
	miguAndroidVersion = "7.41.13"
)

var miguToneExtensions = map[string]string{
	"LQ":   "mp3",
	"PQ":   "mp3",
	"HQ":   "mp3",
	"SQ":   "flac",
	"ZQ":   "flac",
	"ZQ24": "flac",
	"ZQ32": "wav",
	"Z3D":  "wav",
	"I3D":  "m4a",
	"3D60": "wav",
}

var miguTonePaths = map[string]string{
	"LQ":   "全曲试听/Mp3_64_22_16",
	"PQ":   "标清高清/MP3_128_16_Stero",
	"HQ":   "标清高清/MP3_320_16_Stero",
	"SQ":   "歌曲下载/flac",
	"ZQ":   "歌曲下载/flac_24bit",
	"ZQ24": "歌曲下载/flac_24bit",
	"ZQ32": "歌曲下载/wav_32bit",
}

// miguEncryptedAppCode is decrypted by the Android client with the first 16
// characters of Signature.toCharsString() as the AES-128-CBC key.
var miguEncryptedAppCode = []byte{
	0xAE, 0xEA, 0xC7, 0x17, 0x05, 0xA6, 0x89, 0x55,
	0xA9, 0x31, 0xA6, 0xFE, 0x01, 0xE7, 0x28, 0xFF,
	0xF6, 0x49, 0x71, 0xE6, 0x1C, 0xEA, 0xF0, 0x0A,
	0xE3, 0xEA, 0x43, 0xC3, 0xE0, 0x58, 0x33, 0x83,
}

// DownloadInfo describes a Migu audio stream. Z3D streams require
// DecryptAudio with FileKey before they can be played or saved as audio.
type DownloadInfo struct {
	URL       string
	Format    string
	Ext       string
	Encrypted bool
	FileKey   string
	Size      int64
}

func GetDownloadInfo(s *model.Song) (*DownloadInfo, error) {
	return defaultMigu.GetDownloadInfo(s)
}

func GetDownloadURL(s *model.Song) (string, error) {
	return defaultMigu.GetDownloadURL(s)
}

func (m *Migu) GetDownloadInfo(song *model.Song) (*DownloadInfo, error) {
	if song == nil {
		return nil, errors.New("song is nil")
	}
	if song.Source != "" && song.Source != "migu" {
		return nil, errors.New("source mismatch")
	}

	contentID, resourceType, formatType := miguSongParts(song)
	if contentID == "" {
		return nil, errors.New("invalid id structure and missing content id")
	}
	if resourceType == "" {
		resourceType = "2"
	}

	targetFormat := normalizeMiguTone(formatType)
	if targetFormat == "" {
		targetFormat = normalizeMiguTone(song.Ext)
	}
	if targetFormat == "" {
		targetFormat = "PQ"
	}

	copyrightID := ""
	songID := ""
	albumID := strings.TrimSpace(song.AlbumID)
	if song.Extra != nil {
		copyrightID = strings.TrimSpace(song.Extra["copyright_id"])
		songID = strings.TrimSpace(song.Extra["song_id"])
		if extraAlbumID := strings.TrimSpace(song.Extra["album_id"]); extraAlbumID != "" {
			albumID = extraAlbumID
		}
	}

	requestTones := []string{"ZQ32", targetFormat, "PQ"}
	responses := make([]*miguListenResponse, 0, len(requestTones))
	seenTones := make(map[string]struct{}, len(requestTones))
	var fetchErr error
	for _, tone := range requestTones {
		if _, ok := seenTones[tone]; ok {
			continue
		}
		seenTones[tone] = struct{}{}

		resp, err := m.fetchListenInfo(contentID, copyrightID, songID, albumID, resourceType, tone)
		if err == nil {
			responses = append(responses, resp)
		} else if fetchErr == nil {
			fetchErr = err
		}
	}

	candidates := buildMiguDownloadCandidates(responses, song.Size)
	if info := m.selectHighestValidDownload(candidates); info != nil {
		return info, nil
	}
	if len(candidates) == 0 {
		for _, resp := range responses {
			if message := strings.TrimSpace(miguListenResponseMessage(resp)); message != "empty download url" {
				return nil, errors.New(message)
			}
		}
		if fetchErr != nil {
			return nil, fetchErr
		}
		return nil, errors.New("migu returned no download quality")
	}
	if fetchErr != nil {
		return nil, fmt.Errorf("migu returned no accessible download quality: %w", fetchErr)
	}
	return nil, errors.New("migu returned no accessible download quality")
}

func (m *Migu) GetDownloadURL(song *model.Song) (string, error) {
	info, err := m.GetDownloadInfo(song)
	if err != nil {
		return "", err
	}
	return info.URL, nil
}

func miguSongParts(song *model.Song) (contentID, resourceType, formatType string) {
	if song == nil {
		return "", "", ""
	}
	if song.Extra != nil {
		contentID = strings.TrimSpace(song.Extra["content_id"])
		resourceType = strings.TrimSpace(song.Extra["resource_type"])
		formatType = strings.TrimSpace(song.Extra["format_type"])
	}
	if contentID != "" && resourceType != "" && formatType != "" {
		return contentID, resourceType, formatType
	}

	parts := strings.Split(song.ID, "|")
	if len(parts) == 3 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2])
	}
	if contentID == "" {
		contentID = strings.TrimSpace(song.ID)
	}
	return contentID, resourceType, formatType
}

type miguListenResponse struct {
	Code string `json:"code"`
	Info string `json:"info"`
	Data struct {
		URL        string `json:"url"`
		FormatType string `json:"formatType"`
		DialogInfo struct {
			Text string `json:"text"`
		} `json:"dialogInfo"`
		SongItem MiguSongItem `json:"songItem"`
	} `json:"data"`
}

func (m *Migu) fetchListenInfo(contentID, copyrightID, songID, albumID, resourceType, toneFlag string) (*miguListenResponse, error) {
	params := url.Values{}
	params.Set("netType", "01")
	params.Set("resourceType", firstNonEmpty(resourceType, "2"))
	params.Set("contentId", strings.TrimSpace(contentID))
	params.Set("toneFlag", firstNonEmpty(toneFlag, "PQ"))
	if strings.TrimSpace(copyrightID) != "" {
		params.Set("copyrightId", strings.TrimSpace(copyrightID))
	}
	if strings.TrimSpace(songID) != "" {
		params.Set("songId", strings.TrimSpace(songID))
	}
	if strings.TrimSpace(albumID) != "" {
		params.Set("albumId", strings.TrimSpace(albumID))
	}

	req, err := http.NewRequest(http.MethodGet, miguListenURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", miguAndroidUA)
	req.Header.Set("channel", miguAndroidChannel)
	req.Header.Set("version", miguAndroidVersion)
	req.Header.Set("Referer", "https://music.migu.cn/")
	if cookie := strings.TrimSpace(m.cookie); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("migu listen endpoint returned status %d", resp.StatusCode)
	}

	var result miguListenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("migu listen response json parse error: %w", err)
	}
	if result.Code != "" && result.Code != "000000" {
		return nil, fmt.Errorf("migu api error: %s (code %s)", result.Info, result.Code)
	}
	return &result, nil
}

func downloadInfoFromResponse(resp *miguListenResponse, targetFormat string, expectedSize int64) *DownloadInfo {
	if resp == nil {
		return nil
	}
	if info := embeddedDownloadInfoFromResponse(resp, targetFormat, expectedSize); info != nil {
		return info
	}
	rawURL := normalizeMiguDownloadURL(resp.Data.URL)
	if rawURL == "" {
		return nil
	}
	format := firstNonEmpty(resp.Data.FormatType, targetFormat)
	if isEncryptedMiguDirectURL(rawURL) {
		return nil
	}
	info, _ := miguDownloadInfoForTone(rawURL, format, targetFormat, expectedSize)
	return info
}

var miguTonePreference = []string{
	"ZQ32",
	"ZQ24",
	"ZQ",
	"SQ",
	"HQ",
	"Z3D",
	"I3D",
	"3D60",
	"PQ",
	"LQ",
}

func buildMiguDownloadCandidates(responses []*miguListenResponse, expectedSize int64) []*DownloadInfo {
	type directSource struct {
		url    string
		format string
	}
	sources := make([]directSource, 0, len(responses))
	seenSources := make(map[string]struct{}, len(responses))
	for _, resp := range responses {
		if resp == nil {
			continue
		}
		if candidateURL := normalizeMiguDownloadURL(resp.Data.URL); candidateURL != "" {
			actualFormat := firstNonEmpty(normalizeMiguTone(resp.Data.FormatType), "PQ")
			key := actualFormat + "\x00" + candidateURL
			if _, ok := seenSources[key]; !ok {
				seenSources[key] = struct{}{}
				sources = append(sources, directSource{url: candidateURL, format: actualFormat})
			}
		}
	}

	candidates := make([]*DownloadInfo, 0, len(miguTonePreference))
	seen := make(map[string]struct{})
	addCandidate := func(info *DownloadInfo) {
		if info == nil || strings.TrimSpace(info.URL) == "" {
			return
		}
		key := info.Format + "\x00" + info.URL
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		candidates = append(candidates, info)
	}

	for _, tone := range miguTonePreference {
		for _, resp := range responses {
			addCandidate(embeddedDownloadInfoFromResponse(resp, tone, expectedSize))
		}
		for _, source := range sources {
			if tone == source.format {
				if info, err := directDownloadInfo(source.url, source.format, expectedSize); err == nil {
					addCandidate(info)
				}
				continue
			}
			rewrittenURL, ok := rewriteMiguToneURL(source.url, tone)
			if !ok {
				continue
			}
			if info, err := directDownloadInfo(rewrittenURL, tone, 0); err == nil {
				addCandidate(info)
			}
		}
	}
	return candidates
}

func (m *Migu) selectHighestValidDownload(candidates []*DownloadInfo) *DownloadInfo {
	for _, candidate := range candidates {
		if m.downloadInfoValid(candidate) {
			return candidate
		}
	}
	return nil
}

func (m *Migu) downloadInfoValid(info *DownloadInfo) bool {
	if info == nil || strings.TrimSpace(info.URL) == "" {
		return false
	}

	req, err := http.NewRequest(http.MethodGet, info.URL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", miguAndroidUA)
	req.Header.Set("Referer", "https://music.migu.cn/")
	req.Header.Set("Range", "bytes=0-63")
	if cookie := strings.TrimSpace(m.cookie); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return false
	}

	prefix, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return false
	}
	if info.Encrypted {
		if _, err := DecryptAudio(prefix, info.FileKey); err != nil {
			return false
		}
	} else if !isMiguAudioMagic(prefix) {
		return false
	}

	if size := miguResponseSize(resp); size > 0 {
		info.Size = size
	}
	return true
}

func embeddedDownloadInfoFromResponse(resp *miguListenResponse, targetFormat string, expectedSize int64) *DownloadInfo {
	if resp == nil {
		return nil
	}
	code := resp.Data.SongItem.Z3DCode

	switch targetFormat {
	case "Z3D":
		rawURL := normalizeMiguDownloadURL(firstNonEmpty(code.AndroidURL, code.URL))
		fileKey := strings.TrimSpace(code.AndroidFileKey)
		if rawURL == "" || fileKey == "" {
			return nil
		}
		return &DownloadInfo{
			URL:       rawURL,
			Format:    "Z3D",
			Ext:       firstNonEmpty(strings.TrimPrefix(code.AndroidFileType, "."), "wav"),
			Encrypted: true,
			FileKey:   fileKey,
			Size:      firstPositiveInt64(parseMiguSize(code.AndroidSize), expectedSize),
		}
	case "I3D":
		rawURL := normalizeMiguDownloadURL(firstNonEmpty(code.IOSURL, code.URL))
		fileKey := strings.TrimSpace(code.IOSFileKey)
		if rawURL == "" || fileKey == "" {
			return nil
		}
		return &DownloadInfo{
			URL:       rawURL,
			Format:    "I3D",
			Ext:       firstNonEmpty(strings.TrimPrefix(code.IOSFileType, "."), "m4a"),
			Encrypted: true,
			FileKey:   fileKey,
			Size:      firstPositiveInt64(parseMiguSize(code.IOSSize), expectedSize),
		}
	case "3D60":
		rawURL := normalizeMiguDownloadURL(firstNonEmpty(code.URL, code.H5URL))
		if rawURL == "" {
			return nil
		}
		return &DownloadInfo{
			URL:    rawURL,
			Format: "3D60",
			Ext:    firstNonEmpty(strings.TrimPrefix(code.FileType, "."), "wav"),
			Size:   firstPositiveInt64(parseMiguSize(code.Size), expectedSize),
		}
	default:
		return nil
	}
}

func directDownloadInfo(rawURL, format string, expectedSize int64) (*DownloadInfo, error) {
	rawURL = normalizeMiguDownloadURL(rawURL)
	if rawURL == "" {
		return nil, errors.New("empty download url")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid migu download url: %w", err)
	}
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(parsed.Path)), ".")
	format = normalizeMiguTone(format)
	if ext == "" {
		ext = miguToneExtensions[format]
	}
	if ext == "" {
		ext = "mp3"
	}
	if format == "" {
		format = formatFromMiguExt(ext)
	}

	return &DownloadInfo{
		URL:    rawURL,
		Format: format,
		Ext:    ext,
		Size:   expectedSize,
	}, nil
}

func miguDownloadInfoForTone(rawURL, actualFormat, targetFormat string, expectedSize int64) (*DownloadInfo, error) {
	actualFormat = normalizeMiguTone(actualFormat)
	targetFormat = normalizeMiguTone(targetFormat)
	if targetFormat != "" && targetFormat != actualFormat {
		if rewrittenURL, ok := rewriteMiguToneURL(rawURL, targetFormat); ok {
			return directDownloadInfo(rewrittenURL, targetFormat, expectedSize)
		}
	}
	return directDownloadInfo(rawURL, firstNonEmpty(actualFormat, targetFormat), expectedSize)
}

func rewriteMiguToneURL(rawURL, targetFormat string) (string, bool) {
	targetFormat = normalizeMiguTone(targetFormat)
	targetPath := miguTonePaths[targetFormat]
	targetExt := miguToneExtensions[targetFormat]
	if targetPath == "" || targetExt == "" || targetFormat == "Z3D" || targetFormat == "I3D" || targetFormat == "3D60" {
		return "", false
	}

	parsed, err := url.Parse(normalizeMiguDownloadURL(rawURL))
	if err != nil {
		return "", false
	}
	sourcePath := detectMiguTonePath(parsed.Path)
	if sourcePath == "" {
		return "", false
	}

	newPath := strings.Replace(parsed.Path, sourcePath, targetPath, 1)
	if newPath == parsed.Path {
		return "", false
	}
	if oldExt := path.Ext(newPath); oldExt != "" {
		newPath = strings.TrimSuffix(newPath, oldExt)
	}
	newPath += "." + targetExt
	parsed.Path = newPath
	parsed.RawPath = ""
	return parsed.String(), true
}

func detectMiguTonePath(decodedPath string) string {
	for _, format := range []string{"ZQ24", "ZQ", "ZQ32", "SQ", "HQ", "LQ", "PQ"} {
		tonePath := miguTonePaths[format]
		if tonePath != "" && strings.Contains(decodedPath, tonePath) {
			return tonePath
		}
	}
	return ""
}

func normalizeMiguDownloadURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if strings.HasPrefix(rawURL, "//") {
		return "https:" + rawURL
	}
	if strings.HasPrefix(rawURL, "ftp://218.200.160.122:21/") {
		return "https://freetyst.nf.migu.cn/" + strings.TrimPrefix(rawURL, "ftp://218.200.160.122:21/")
	}
	return rawURL
}

func isEncryptedMiguDirectURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	unescapedPath, err := url.PathUnescape(parsed.EscapedPath())
	if err != nil {
		unescapedPath = parsed.Path
	}
	return strings.Contains(unescapedPath, "wav_3d/") && !strings.Contains(unescapedPath, "wav_3d_60s/")
}

func miguListenResponseMessage(resp *miguListenResponse) string {
	if resp == nil {
		return "empty download url"
	}
	if message := strings.TrimSpace(resp.Data.DialogInfo.Text); message != "" {
		return message
	}
	if message := strings.TrimSpace(resp.Info); message != "" {
		return message
	}
	return "empty download url"
}

func normalizeMiguTone(value string) string {
	value = strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(value, ".")))
	if _, ok := miguToneExtensions[value]; ok {
		return value
	}
	return ""
}

func formatFromMiguExt(ext string) string {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case "flac":
		return "SQ"
	case "wav":
		return "ZQ32"
	case "m4a":
		return "I3D"
	default:
		return "PQ"
	}
}

func parseMiguSize(value string) int64 {
	size, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if size < 0 {
		return 0
	}
	return size
}

func firstPositiveInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

// DecryptAudio decrypts a Migu Z3D stream. The stream key is the uppercase
// hexadecimal MD5 of the decrypted app code followed by androidFileKey. Each
// byte is decoded as cipher[i] - streamKey[i % 32].
func DecryptAudio(fileData []byte, fileKey string) ([]byte, error) {
	if len(fileData) == 0 {
		return nil, errors.New("empty z3d data")
	}
	if isMiguAudioMagic(fileData) {
		return fileData, nil
	}

	payload := fileData
	if bytes.HasPrefix(payload, []byte("Z3D")) {
		payload = payload[3:]
	}
	if len(payload) == 0 {
		return nil, errors.New("empty z3d payload")
	}

	streamKey, err := miguZ3DStreamKey(fileKey)
	if err != nil {
		return nil, err
	}
	plain := make([]byte, len(payload))
	for i, value := range payload {
		plain[i] = value - streamKey[i&31]
	}
	if !isMiguAudioMagic(plain) {
		return nil, errors.New("z3d decryption produced an invalid audio header")
	}
	return plain, nil
}

func miguZ3DStreamKey(fileKey string) ([]byte, error) {
	fileKey = strings.TrimSpace(fileKey)
	if fileKey == "" {
		return nil, errors.New("empty z3d file key")
	}

	block, err := aes.NewCipher([]byte(miguAndroidAppKey))
	if err != nil {
		return nil, err
	}
	appCode := make([]byte, len(miguEncryptedAppCode))
	iv := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(appCode, miguEncryptedAppCode)

	hash := md5.New()
	_, _ = hash.Write(appCode)
	_, _ = hash.Write([]byte(fileKey))
	sum := hash.Sum(nil)
	encoded := make([]byte, hex.EncodedLen(len(sum)))
	hex.Encode(encoded, sum)
	for i := range encoded {
		if encoded[i] >= 'a' && encoded[i] <= 'f' {
			encoded[i] -= 'a' - 'A'
		}
	}
	return encoded, nil
}

func isMiguAudioMagic(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	header := data[:4]
	return bytes.Equal(header, []byte("fLaC")) ||
		(bytes.Equal(header, []byte("RIFF")) && len(data) >= 12 && bytes.Equal(data[8:12], []byte("WAVE"))) ||
		bytes.Equal(header, []byte("OggS")) ||
		bytes.Equal(data[:3], []byte("ID3")) ||
		(len(data) >= 12 && bytes.Equal(data[4:8], []byte("ftyp"))) ||
		(data[0] == 0xFF && data[1]&0xE0 == 0xE0)
}

func miguResponseSize(resp *http.Response) int64 {
	if resp == nil {
		return 0
	}
	if contentRange := strings.TrimSpace(resp.Header.Get("Content-Range")); contentRange != "" {
		if slash := strings.LastIndexByte(contentRange, '/'); slash >= 0 {
			if size, err := strconv.ParseInt(strings.TrimSpace(contentRange[slash+1:]), 10, 64); err == nil && size > 0 {
				return size
			}
		}
	}
	if resp.StatusCode == http.StatusOK && resp.ContentLength > 0 {
		return resp.ContentLength
	}
	return 0
}
