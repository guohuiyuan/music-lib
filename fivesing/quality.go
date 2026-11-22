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

// ListQualityOptions 返回 5sing 支持的 MP3 档位。
func ListQualityOptions() []model.AudioQualityOption {
	options := make([]model.AudioQualityOption, 0, len(fivesingQualityCandidates))
	for _, candidate := range fivesingQualityCandidates {
		options = append(options, candidate.option)
	}
	return options
}

// AvailableQualityOptions 探测一个稳定的 5sing 歌曲，并返回当前请求
// 实际能解析到的档位。
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

// GetDownloadURLWithQuality 解析指定的 5sing 档位。它不会静默替换成其他档位，
// 这部分由公共核心的回退逻辑处理。
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
