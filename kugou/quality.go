package kugou

import (
	"errors"
	"fmt"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
)

var kugouQualityCandidates = []model.AudioQualityOption{
	{Value: model.AudioQualityHiRes, Label: "Hi-Res 无损", Format: "flac", VIP: true},
	{Value: model.AudioQualityLossless, Label: "无损音质", Format: "flac", VIP: true},
	{Value: model.AudioQualityHigh, Label: "高品质 320k", Format: "mp3", Bitrate: 320},
	{Value: model.AudioQualityHigher, Label: "较高品质 192k", Format: "ogg", Bitrate: 192},
	{Value: model.AudioQualityStandard, Label: "标准音质 128k", Format: "mp3", Bitrate: 128},
	{Value: model.AudioQualityLow, Label: "低品质 64k", Format: "mp3", Bitrate: 64},
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

// ListQualityOptions returns all Kugou quality tiers.
func ListQualityOptions() []model.AudioQualityOption {
	return append([]model.AudioQualityOption(nil), kugouQualityCandidates...)
}

// AvailableQualityOptions probes actual download hashes for the configured
// account. A song may have fewer tiers, so callers should still handle a
// per-song quality error.
func (k *Kugou) AvailableQualityOptions() ([]model.AudioQualityOption, error) {
	if k.availableQualityCache != nil {
		return append([]model.AudioQualityOption(nil), k.availableQualityCache...), nil
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

	options := probeKugouQualityOptions(
		probeSongs,
		func(song *model.Song, quality model.AudioQuality) bool {
			_, err := k.GetDownloadURLWithQuality(song, string(quality))
			return err == nil
		},
	)
	k.availableQualityCache = append([]model.AudioQualityOption(nil), options...)
	return append([]model.AudioQualityOption(nil), options...), nil
}

// probeKugouQualityOptions uses one representative song at a time and stops
// after the first song that yields a playable quality set. Quality is
// track-specific, so probing every missing tier against many unrelated songs
// adds latency without making the result more reliable.
func probeKugouQualityOptions(
	songs []*model.Song,
	probe func(*model.Song, model.AudioQuality) bool,
) []model.AudioQualityOption {
	for _, song := range songs {
		if song == nil {
			continue
		}
		available := make(map[model.AudioQuality]struct{}, len(kugouQualityCandidates))
		for _, option := range kugouQualityCandidates {
			if probe(song, option.Value) {
				available[option.Value] = struct{}{}
			}
		}
		if len(available) == 0 {
			continue
		}
		return qualityOptionsFromAvailable(available)
	}
	return nil
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
		info, err := k.fetchKugouQualityInfo(candidate, hash)
		if err != nil {
			lastErr = err
			continue
		}
		if info == nil || strings.TrimSpace(info.URL) == "" {
			continue
		}
		if kugouSongQuality(info) != normalized {
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

func (k *Kugou) fetchKugouQualityInfo(song *model.Song, hash string) (*model.Song, error) {
	var lastErr error
	if info, err := k.fetchVIPSongInfo(song); err == nil && info != nil {
		return info, nil
	} else if err != nil {
		lastErr = err
	}
	if info, err := k.fetchSonginfoV2(hash); err == nil && info != nil {
		return info, nil
	} else if err != nil {
		lastErr = err
	}
	if info, err := k.fetchTrackerSongInfo(hash); err == nil && info != nil {
		return info, nil
	} else if err != nil {
		lastErr = err
	}
	if info, err := k.fetchSongInfo(hash); err == nil && info != nil {
		return info, nil
	} else if err != nil {
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("kugou download url not found")
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
