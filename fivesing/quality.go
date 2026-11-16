package fivesing

import (
	"errors"
	"fmt"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
)

type fivesingQualityCandidate struct {
	option model.AudioQualityOption
	tier   string
}

var fivesingQualityCandidates = []fivesingQualityCandidate{
	{
		option: model.AudioQualityOption{
			Value:   model.AudioQualityHigh,
			Label:   "高品质",
			Format:  "mp3",
			Bitrate: 320,
		},
		tier: "sq",
	},
	{
		option: model.AudioQualityOption{
			Value:   model.AudioQualityHigher,
			Label:   "较高品质",
			Format:  "mp3",
			Bitrate: 192,
		},
		tier: "hq",
	},
	{
		option: model.AudioQualityOption{
			Value:   model.AudioQualityStandard,
			Label:   "标准音质",
			Format:  "mp3",
			Bitrate: 128,
		},
		tier: "lq",
	},
}

// ListQualityOptions returns the MP3 tiers exposed by 5sing.
func ListQualityOptions() []model.AudioQualityOption {
	options := make([]model.AudioQualityOption, 0, len(fivesingQualityCandidates))
	for _, candidate := range fivesingQualityCandidates {
		options = append(options, candidate.option)
	}
	return options
}

// AvailableQualityOptions probes a stable 5sing track and reports tiers that
// the current request can actually resolve.
func (f *Fivesing) AvailableQualityOptions() ([]model.AudioQualityOption, error) {
	const (
		probeSongID   = "3529943"
		probeSongType = "yc"
	)

	links, err := f.fetchAudioLinks(probeSongID, probeSongType)
	if err != nil {
		return nil, err
	}

	options := make([]model.AudioQualityOption, 0, len(fivesingQualityCandidates))
	for _, candidate := range fivesingQualityCandidates {
		if links.urlFor(candidate.tier) != "" {
			options = append(options, candidate.option)
		}
	}
	if len(options) == 0 {
		return nil, errors.New("5sing returned no playable quality")
	}
	return options, nil
}

// GetDownloadURLWithQuality resolves an explicit 5sing tier. It does not
// silently substitute another tier; the shared core fallback handles that.
func (f *Fivesing) GetDownloadURLWithQuality(s *model.Song, quality string) (string, error) {
	normalized, ok := model.NormalizeAudioQuality(quality)
	if !ok {
		return "", fmt.Errorf("unsupported 5sing audio quality: %s", quality)
	}
	if s == nil {
		return "", errors.New("song is nil")
	}
	if !strings.EqualFold(strings.TrimSpace(s.Source), "fivesing") {
		return "", errors.New("source mismatch")
	}
	if normalized == model.AudioQualityAuto {
		return f.GetDownloadURL(s)
	}

	candidate, ok := fivesingQualityCandidateFor(normalized)
	if !ok {
		return "", fmt.Errorf("unsupported 5sing audio quality: %s", quality)
	}
	songID, songType, err := fivesingSongIdentity(s)
	if err != nil {
		return "", err
	}

	links, err := f.fetchAudioLinks(songID, songType)
	if err != nil {
		return "", err
	}
	streamURL := links.urlFor(candidate.tier)
	if streamURL == "" {
		return "", fmt.Errorf("5sing audio quality %s is unavailable", normalized)
	}
	return streamURL, nil
}

func fivesingQualityCandidateFor(quality model.AudioQuality) (fivesingQualityCandidate, bool) {
	for _, candidate := range fivesingQualityCandidates {
		if candidate.option.Value == quality {
			return candidate, true
		}
	}
	return fivesingQualityCandidate{}, false
}

func fivesingSongIdentity(song *model.Song) (string, string, error) {
	if song == nil {
		return "", "", errors.New("song is nil")
	}

	var songID, songType string
	if song.Extra != nil {
		songID = strings.TrimSpace(song.Extra["songid"])
		songType = strings.TrimSpace(song.Extra["songtype"])
	}
	if songID == "" || songType == "" {
		parts := strings.Split(song.ID, "|")
		if len(parts) != 2 {
			return "", "", errors.New("invalid id structure")
		}
		if songID == "" {
			songID = strings.TrimSpace(parts[0])
		}
		if songType == "" {
			songType = strings.TrimSpace(parts[1])
		}
	}
	if songID == "" || songType == "" {
		return "", "", errors.New("invalid id structure")
	}
	return songID, songType, nil
}
