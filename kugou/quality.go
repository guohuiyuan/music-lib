package kugou

import (
	"errors"
	"fmt"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
)

var kugouQualityCandidates = []model.AudioQualityOption{
	{Value: model.AudioQualityUltra, Label: "超清 96kHz", Format: "flac", VIP: true},
	{Value: model.AudioQualitySpatial, Label: "沉浸环绕声 6ch", Format: "flac", VIP: true},
	{Value: model.AudioQualityHiRes, Label: "Hi-Res 无损", Format: "flac", VIP: true},
	{Value: model.AudioQualityLossless, Label: "无损音质", Format: "flac", VIP: true},
	{Value: model.AudioQualityHigh, Label: "高品质 320k", Format: "mp3", Bitrate: 320},
	{Value: model.AudioQualityStandard, Label: "标准音质 128k", Format: "mp3", Bitrate: 128},
}

var kugouQualityProbeSong = &model.Song{
	Source:  "kugou",
	ID:      "83368470292244265486BF864701222C",
	AlbumID: "82564821",
	Extra: map[string]string{
		"hash":           "83368470292244265486BF864701222C",
		"ogg_320_hash":   "878D0D6E847F402299D29D4295BC21CB",
		"ogg_128_hash":   "A76E5E3C04E98737DBDD0C8C550EDAE5",
		"sq_hash":        "83368470292244265486BF864701222C",
		"file_hash":      "AB05B8F658851282DCB2CBAD548AEB9B",
		"res_hash":       "983AA9AF17E1688CC70EDEAAF6256D5B",
		"hq_hash":        "7DD13522B6C143A39E10507B74B0A876",
		"album_audio_id": "590964610",
		"album_id":       "82564821",
		"privilege":      "10",
	},
}

const kugouQualityProbeKeyword = "痛仰乐队"

// ListQualityOptions 返回酷狗全部音质档位。
func ListQualityOptions() []model.AudioQualityOption {
	return append([]model.AudioQualityOption(nil), kugouQualityCandidates...)
}

// AvailableQualityOptions 针对配置的账号探测实际可用的下载 hash。
// 单首歌可能少于全部档位，调用方仍需处理单曲音质错误。
func (k *Kugou) AvailableQualityOptions() ([]model.AudioQualityOption, error) {
	if k.availableQualityCache != nil {
		return append([]model.AudioQualityOption(nil), k.availableQualityCache...), nil
	}

	if !k.hasAppToken() && !k.hasSonginfoAuth() {
		// 没有可用登录态时酷狗匿名只保证标准音质，直接返回可避免长时间探测。
		options := kugouQualityOptionsFor(model.AudioQualityStandard)
		k.availableQualityCache = append([]model.AudioQualityOption(nil), options...)
		return append([]model.AudioQualityOption(nil), options...), nil
	}

	probeSongs := make([]*model.Song, 0, 4)
	if songs, err := k.Search(kugouQualityProbeKeyword); err == nil {
		for i := range songs {
			if i >= 3 {
				break
			}
			song := songs[i]
			probeSongs = append(probeSongs, &song)
		}
	}
	probeSongs = append(probeSongs, kugouQualityProbeSong)

	probe := func(song *model.Song, quality model.AudioQuality) bool {
		_, err := k.GetDownloadURLWithQuality(song, string(quality))
		return err == nil
	}
	if k.hasAppToken() {
		// App token 账号直接走 v5 精确探测，避免逐档回退到大体积接口。
		probe = func(song *model.Song, quality model.AudioQuality) bool {
			return k.probeV5Quality(song, quality)
		}
	} else {
		// 普通网页 Cookie 只用一个探测歌曲，避免逐个档位回退导致设置页长时间等待。
		probeSongs = probeSongs[:1]
	}

	options := probeKugouQualityOptions(probeSongs, probe)
	k.availableQualityCache = append([]model.AudioQualityOption(nil), options...)
	return append([]model.AudioQualityOption(nil), options...), nil
}

func (k *Kugou) hasAppToken() bool {
	cookie := parseKugouCookie(k.cookie)
	return strings.TrimSpace(cookie["token"]) != "" &&
		strings.TrimSpace(cookie["userid"]) != "" &&
		strings.TrimSpace(cookie["KUGOU_API_MID"]) != ""
}

func (k *Kugou) hasSonginfoAuth() bool {
	cookie := parseKugouCookie(k.cookie)
	return strings.TrimSpace(cookie["t"]) != "" && strings.TrimSpace(cookie["KugooID"]) != ""
}

func (k *Kugou) probeV5Quality(song *model.Song, quality model.AudioQuality) bool {
	for _, hash := range kugouHashesForQuality(song, quality) {
		for _, v5Quality := range kugouV5QualityNames(quality) {
			info, err := k.fetchURLV5WithQuality(song, hash, v5Quality)
			if err == nil && kugouQualitySatisfies(info, quality) {
				return true
			}
		}
	}
	return false
}

func kugouQualityOptionsFor(quality model.AudioQuality) []model.AudioQualityOption {
	options := make([]model.AudioQualityOption, 0, 1)
	for _, option := range kugouQualityCandidates {
		if option.Value == quality {
			options = append(options, option)
		}
	}
	return options
}

// probeKugouQualityOptions 会在多首探测歌曲之间补齐尚未确认的档位。
// 单首歌可能缺少某些 hash（Hi-Res/环绕声尤其明显），只看第一首歌会漏报。
func probeKugouQualityOptions(
	songs []*model.Song,
	probe func(*model.Song, model.AudioQuality) bool,
) []model.AudioQualityOption {
	available := make(map[model.AudioQuality]struct{}, len(kugouQualityCandidates))
	for _, song := range songs {
		if song == nil {
			continue
		}
		for _, option := range kugouQualityCandidates {
			if _, exists := available[option.Value]; exists {
				continue
			}
			if probe(song, option.Value) {
				available[option.Value] = struct{}{}
			}
		}
		if len(available) == len(kugouQualityCandidates) {
			break
		}
	}
	if len(available) == 0 {
		return nil
	}
	return qualityOptionsFromAvailable(available)
}

func qualityOptionsFromAvailable(available map[model.AudioQuality]struct{}) []model.AudioQualityOption {
	options := make([]model.AudioQualityOption, 0, len(available))
	for _, option := range kugouQualityCandidates {
		if _, ok := available[option.Value]; ok {
			options = append(options, option)
		}
	}
	return options
}

func (k *Kugou) GetDownloadURLWithQuality(s *model.Song, quality string) (string, error) {
	normalized, ok := model.NormalizeAudioQuality(quality)
	if !ok {
		return "", fmt.Errorf("unsupported kugou audio quality: %s", quality)
	}
	if normalized == model.AudioQualityAuto {
		return k.GetDownloadURL(s)
	}
	if s == nil {
		return "", errors.New("song is nil")
	}
	if s.Source != "kugou" {
		return "", errors.New("source mismatch")
	}

	hashes := kugouHashesForQuality(s, normalized)
	if len(hashes) == 0 {
		return "", fmt.Errorf("kugou audio quality %s is unavailable for this song", normalized)
	}

	var lastErr error
	for _, hash := range hashes {
		candidate := &model.Song{
			Source:   "kugou",
			ID:       hash,
			Name:     s.Name,
			Artist:   s.Artist,
			Album:    s.Album,
			AlbumID:  s.AlbumID,
			Duration: s.Duration,
			Extra: map[string]string{
				"hash":           hash,
				"album_audio_id": firstNonEmpty(extraValue(s, "album_audio_id"), extraValue(s, "audio_id")),
				"album_id":       firstNonEmpty(extraValue(s, "album_id"), s.AlbumID),
				"privilege":      extraValue(s, "privilege"),
			},
		}
		info, err := k.fetchKugouQualityInfo(candidate, hash, normalized)
		if err != nil {
			lastErr = err
			continue
		}
		if info == nil || strings.TrimSpace(info.URL) == "" {
			continue
		}
		if !kugouQualitySatisfies(info, normalized) {
			lastErr = fmt.Errorf("kugou returned %s instead of %s", kugouSongQuality(info), normalized)
			continue
		}
		return info.URL, nil
	}
	if lastErr != nil {
		return "", fmt.Errorf("kugou audio quality %s unavailable: %w", normalized, lastErr)
	}
	return "", fmt.Errorf("kugou audio quality %s is unavailable for this song or account", normalized)
}

func (k *Kugou) fetchKugouQualityInfo(song *model.Song, hash string, quality model.AudioQuality) (*model.Song, error) {
	var lastErr error
	for _, v5Quality := range kugouV5QualityNames(quality) {
		info, err := k.fetchURLV5WithQuality(song, hash, v5Quality)
		if err == nil && info != nil && info.URL != "" {
			if kugouQualitySatisfies(info, quality) {
				return info, nil
			}
			lastErr = fmt.Errorf("kugou returned %s instead of %s", kugouSongQuality(info), quality)
		} else if err != nil {
			lastErr = err
		}
	}
	for _, fetch := range []func(string) (*model.Song, error){
		k.fetchSonginfoV2,
		k.fetchTrackerSongInfo,
		k.fetchSongInfo,
	} {
		info, err := fetch(hash)
		if err == nil && info != nil && info.URL != "" {
			if kugouQualitySatisfies(info, quality) {
				return info, nil
			}
			lastErr = fmt.Errorf("kugou returned %s instead of %s", kugouSongQuality(info), quality)
		} else if err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("kugou download url not found")
}

func kugouV5QualityNames(quality model.AudioQuality) []string {
	switch quality {
	case model.AudioQualityUltra:
		return []string{"viper_clear"}
	case model.AudioQualitySpatial:
		return []string{"viper_atmos"}
	case model.AudioQualityHiRes:
		return []string{"high", "viper_clear"}
	case model.AudioQualityLossless:
		return []string{"flac"}
	case model.AudioQualityHigh:
		return []string{"320"}
	case model.AudioQualityHigher:
		return []string{"192"}
	case model.AudioQualityStandard:
		return []string{"128"}
	case model.AudioQualityLow:
		return []string{"64"}
	}
	return nil
}

func kugouQualitySatisfies(info *model.Song, quality model.AudioQuality) bool {
	if info == nil || strings.TrimSpace(info.URL) == "" {
		return false
	}
	actual := kugouSongQuality(info)
	if actual == quality {
		return true
	}
	switch quality {
	case model.AudioQualityHiRes:
		return actual == model.AudioQualityUltra
	case model.AudioQualitySpatial:
		bitrate := inferKugouBitrate(info.Bitrate, info.Size, info.Duration)
		return strings.EqualFold(strings.TrimSpace(info.Ext), "flac") && bitrate >= 1800 && bitrate < 3000
	}
	return false
}

func kugouHashesForQuality(song *model.Song, quality model.AudioQuality) []string {
	var keys []string
	switch quality {
	case model.AudioQualityLow, model.AudioQualityStandard:
		keys = []string{"ogg_128_hash", "file_hash", "hash"}
	case model.AudioQualityHigher:
		keys = []string{"ogg_320_hash", "hq_hash", "res_hash", "hash"}
	case model.AudioQualityHigh:
		keys = []string{"ogg_320_hash", "hq_hash", "res_hash", "file_hash", "hash"}
	case model.AudioQualityLossless:
		keys = []string{"sq_hash", "hash"}
	case model.AudioQualityHiRes:
		keys = []string{"res_hash", "sq_hash", "hash"}
	case model.AudioQualityUltra:
		keys = []string{"res_hash", "sq_hash", "hash"}
	case model.AudioQualitySpatial:
		keys = []string{"res_hash", "hash"}
	default:
		return nil
	}

	result := make([]string, 0, len(keys)+1)
	seen := make(map[string]struct{}, len(keys)+1)
	for _, key := range keys {
		hash := strings.ToLower(strings.TrimSpace(extraValue(song, key)))
		if !isValidHash(hash) {
			continue
		}
		if _, exists := seen[hash]; exists {
			continue
		}
		seen[hash] = struct{}{}
		result = append(result, hash)
	}
	id := strings.ToLower(strings.TrimSpace(song.ID))
	if isValidHash(id) {
		if _, exists := seen[id]; !exists {
			result = append(result, id)
		}
	}
	return result
}

func kugouSongQuality(song *model.Song) model.AudioQuality {
	if song == nil {
		return ""
	}
	ext := strings.ToLower(strings.TrimSpace(song.Ext))
	bitrate := inferKugouBitrate(song.Bitrate, song.Size, song.Duration)
	switch {
	case ext == "flac" && bitrate >= 3000:
		return model.AudioQualityUltra
	case ext == "flac" && bitrate >= 1000:
		return model.AudioQualityHiRes
	case looksLossless(ext, bitrate, song.Size):
		return model.AudioQualityLossless
	case bitrate >= 280:
		return model.AudioQualityHigh
	case bitrate >= 180:
		return model.AudioQualityHigher
	case bitrate >= 96:
		return model.AudioQualityStandard
	default:
		return model.AudioQualityLow
	}
}

func extraValue(song *model.Song, key string) string {
	if song == nil || song.Extra == nil {
		return ""
	}
	return strings.TrimSpace(song.Extra[key])
}
