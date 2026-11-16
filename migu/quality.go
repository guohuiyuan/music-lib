package migu

import (
	"errors"
	"fmt"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
)

var miguQualityProbeSong = &model.Song{
	Source:  "migu",
	ID:      "600902000006889366|2|HQ",
	AlbumID: "1138997327",
	Extra: map[string]string{
		"content_id":    "600902000006889366",
		"resource_type": "2",
		"format_type":   "HQ",
		"copyright_id":  "60054701923",
	},
}

type miguQualityCandidate struct {
	option model.AudioQualityOption
	tones  []string
}

var miguQualityCandidates = []miguQualityCandidate{
	{option: model.AudioQualityOption{Value: model.AudioQualityMaster, Label: "32bit 母带", Format: "wav", VIP: true}, tones: []string{"ZQ32"}},
	{option: model.AudioQualityOption{Value: model.AudioQualitySpatial, Label: "空间音频", Format: "wav", VIP: true}, tones: []string{"Z3D", "I3D", "3D60"}},
	{option: model.AudioQualityOption{Value: model.AudioQualityHiRes, Label: "24bit 无损", Format: "flac", VIP: true}, tones: []string{"ZQ24", "ZQ"}},
	{option: model.AudioQualityOption{Value: model.AudioQualityLossless, Label: "无损音质", Format: "flac", VIP: true}, tones: []string{"SQ"}},
	{option: model.AudioQualityOption{Value: model.AudioQualityHigh, Label: "高品质 320k", Format: "mp3", Bitrate: 320}, tones: []string{"HQ"}},
	{option: model.AudioQualityOption{Value: model.AudioQualityStandard, Label: "标准音质 128k", Format: "mp3", Bitrate: 128}, tones: []string{"PQ"}},
	{option: model.AudioQualityOption{Value: model.AudioQualityLow, Label: "低品质 64k", Format: "mp3", Bitrate: 64}, tones: []string{"LQ"}},
}

// ListQualityOptions returns the complete Migu quality taxonomy.
func ListQualityOptions() []model.AudioQualityOption {
	options := make([]model.AudioQualityOption, 0, len(miguQualityCandidates))
	for _, candidate := range miguQualityCandidates {
		options = append(options, candidate.option)
	}
	return options
}

// AvailableQualityOptions reports the highest stream the current account can
// obtain from a real Migu track, plus every lower quality in the fallback
// ladder. Migu sometimes exposes FLAC through a rewritten standard URL even
// when an explicit VIP tone request is rejected, so probing the normal
// download path is more accurate than checking each VIP flag independently.
func (m *Migu) AvailableQualityOptions() ([]model.AudioQualityOption, error) {
	if m.availableQualityCache != nil {
		return append([]model.AudioQualityOption(nil), m.availableQualityCache...), nil
	}

	info, err := m.GetDownloadInfo(miguQualityProbeSong)
	if err != nil {
		return nil, err
	}
	options := miguQualityOptionsFromTone(info.Format)
	if len(options) == 0 {
		return nil, fmt.Errorf("migu returned unsupported quality %q", info.Format)
	}
	m.availableQualityCache = append([]model.AudioQualityOption(nil), options...)
	return append([]model.AudioQualityOption(nil), options...), nil
}

func miguQualityOptionsFromTone(tone string) []model.AudioQualityOption {
	tone = normalizeMiguTone(tone)
	for index, candidate := range miguQualityCandidates {
		if !miguCandidateMatchesTone(candidate, tone) {
			continue
		}
		options := make([]model.AudioQualityOption, 0, len(miguQualityCandidates)-index)
		for _, fallback := range miguQualityCandidates[index:] {
			options = append(options, fallback.option)
		}
		return options
	}
	return nil
}

func miguCandidateForQuality(quality model.AudioQuality) (miguQualityCandidate, bool) {
	for _, candidate := range miguQualityCandidates {
		if candidate.option.Value == quality {
			return candidate, true
		}
	}
	return miguQualityCandidate{}, false
}

func miguCandidateMatchesTone(candidate miguQualityCandidate, tone string) bool {
	tone = normalizeMiguTone(tone)
	for _, expected := range candidate.tones {
		if tone == expected {
			return true
		}
	}
	return false
}

func miguRequestedCandidate(song *model.Song, quality string) (miguQualityCandidate, error) {
	normalized, ok := model.NormalizeAudioQuality(quality)
	if !ok {
		return miguQualityCandidate{}, fmt.Errorf("unsupported migu audio quality: %s", quality)
	}
	candidate, ok := miguCandidateForQuality(normalized)
	if !ok {
		return miguQualityCandidate{}, fmt.Errorf("unsupported migu audio quality: %s", quality)
	}
	return candidate, nil
}

func (m *Migu) GetDownloadInfoWithQuality(song *model.Song, quality string) (*DownloadInfo, error) {
	if strings.EqualFold(strings.TrimSpace(quality), string(model.AudioQualityAuto)) || strings.TrimSpace(quality) == "" {
		return m.GetDownloadInfo(song)
	}
	if song == nil {
		return nil, errors.New("song is nil")
	}
	if song.Source != "" && song.Source != "migu" {
		return nil, errors.New("source mismatch")
	}

	candidate, err := miguRequestedCandidate(song, quality)
	if err != nil {
		return nil, err
	}
	contentID, resourceType, _ := miguSongParts(song)
	if contentID == "" {
		return nil, errors.New("invalid id structure and missing content id")
	}
	if resourceType == "" {
		resourceType = "2"
	}

	copyrightID, songID := "", ""
	albumID := strings.TrimSpace(song.AlbumID)
	if song.Extra != nil {
		copyrightID = strings.TrimSpace(song.Extra["copyright_id"])
		songID = strings.TrimSpace(song.Extra["song_id"])
		if extraAlbumID := strings.TrimSpace(song.Extra["album_id"]); extraAlbumID != "" {
			albumID = extraAlbumID
		}
	}

	requestTones := miguQualityRequestTones(song, candidate)
	responses := make([]*miguListenResponse, 0, len(requestTones))
	var firstErr error
	for _, tone := range requestTones {
		resp, fetchErr := m.fetchListenInfo(contentID, copyrightID, songID, albumID, resourceType, tone)
		if fetchErr == nil {
			responses = append(responses, resp)
		} else if firstErr == nil {
			firstErr = fetchErr
		}
	}
	for _, info := range buildMiguDownloadCandidates(responses, song.Size) {
		if !miguCandidateMatchesTone(candidate, info.Format) {
			continue
		}
		if m.downloadInfoValid(info) {
			return info, nil
		}
	}
	if firstErr != nil {
		return nil, fmt.Errorf("migu quality %s unavailable: %w", candidate.option.Value, firstErr)
	}
	return nil, fmt.Errorf("migu audio quality %s is unavailable for this song or account", candidate.option.Value)
}

func miguQualityRequestTones(song *model.Song, candidate miguQualityCandidate) []string {
	_, _, sourceTone := miguSongParts(song)
	values := make([]string, 0, len(candidate.tones)+3)
	values = append(values, candidate.tones...)
	values = append(values, sourceTone, "HQ", "PQ")

	tones := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		tone := normalizeMiguTone(value)
		if tone == "" {
			continue
		}
		if _, ok := seen[tone]; ok {
			continue
		}
		seen[tone] = struct{}{}
		tones = append(tones, tone)
	}
	return tones
}

func (m *Migu) GetDownloadURLWithQuality(song *model.Song, quality string) (string, error) {
	info, err := m.GetDownloadInfoWithQuality(song, quality)
	if err != nil {
		return "", err
	}
	return info.URL, nil
}
