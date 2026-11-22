package kuwo

import (
	"errors"
	"fmt"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
)

const kuwoQualityProbeRID = "624683929"

type kuwoQualityCandidate struct {
	option model.AudioQualityOption
	br     string
}

var kuwoQualityCandidates = []kuwoQualityCandidate{
	{option: model.AudioQualityOption{Value: model.AudioQualityHiRes, Label: "Hi-Res 无损", Format: "flac"}, br: "2000kflac"},
	{option: model.AudioQualityOption{Value: model.AudioQualityLossless, Label: "无损音质", Format: "flac", VIP: true}, br: "flac"},
	{option: model.AudioQualityOption{Value: model.AudioQualityHigh, Label: "高品质 320k", Format: "mp3", Bitrate: 320}, br: "320kmp3"},
	{option: model.AudioQualityOption{Value: model.AudioQualityStandard, Label: "标准音质 128k", Format: "mp3", Bitrate: 128}, br: "128kmp3"},
}

// ListQualityOptions 返回酷我全部音频档位。
func ListQualityOptions() []model.AudioQualityOption {
	options := make([]model.AudioQualityOption, 0, len(kuwoQualityCandidates))
	for _, candidate := range kuwoQualityCandidates {
		options = append(options, candidate.option)
	}
	return options
}

// AvailableQualityOptions 用一首真实曲库歌曲校验配置的账号，
// 而不是假设每个 VIP 账号都拥有全部档位。
func (k *Kuwo) AvailableQualityOptions() ([]model.AudioQualityOption, error) {
	if k.availableQualityCache != nil {
		return append([]model.AudioQualityOption(nil), k.availableQualityCache...), nil
	}
	options := make([]model.AudioQualityOption, 0, len(kuwoQualityCandidates))
	var firstErr error
	for _, candidate := range kuwoQualityCandidates {
		_, err := k.GetDownloadURLWithQuality(&model.Song{ID: kuwoQualityProbeRID, Source: "kuwo"}, string(candidate.option.Value))
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		options = append(options, candidate.option)
	}
	if len(options) == 0 && firstErr != nil {
		return nil, firstErr
	}
	k.availableQualityCache = append([]model.AudioQualityOption(nil), options...)
	return append([]model.AudioQualityOption(nil), options...), nil
}

func (k *Kuwo) GetDownloadURLWithQuality(s *model.Song, quality string) (string, error) {
	normalized, ok := model.NormalizeAudioQuality(quality)
	if !ok {
		return "", fmt.Errorf("unsupported kuwo audio quality: %s", quality)
	}
	if normalized == model.AudioQualityAuto {
		return k.GetDownloadURL(s)
	}
	if s == nil {
		return "", errors.New("song is nil")
	}
	if s.Source != "kuwo" {
		return "", errors.New("source mismatch")
	}
	candidate, ok := kuwoCandidateForQuality(normalized)
	if !ok {
		return "", fmt.Errorf("unsupported kuwo audio quality: %s", quality)
	}
	rid := strings.TrimSpace(s.ID)
	if s.Extra != nil && strings.TrimSpace(s.Extra["rid"]) != "" {
		rid = strings.TrimSpace(s.Extra["rid"])
	}
	if rid == "" {
		return "", errors.New("kuwo rid is empty")
	}

	info, err := k.fetchKuwoAudioInfo(rid, candidate.br)
	if err != nil {
		return "", fmt.Errorf("kuwo audio quality %s unavailable: %w", normalized, err)
	}
	if actual := kuwoAudioQuality(info.Bitrate, info.Format); actual != normalized {
		return "", fmt.Errorf("kuwo returned %s instead of %s", actual, normalized)
	}
	return info.URL, nil
}

func kuwoCandidateForQuality(quality model.AudioQuality) (kuwoQualityCandidate, bool) {
	for _, candidate := range kuwoQualityCandidates {
		if candidate.option.Value == quality {
			return candidate, true
		}
	}
	return kuwoQualityCandidate{}, false
}

func kuwoAudioQuality(bitrate int, format string) model.AudioQuality {
	format = strings.ToLower(strings.TrimSpace(format))
	if bitrate > 100000 {
		bitrate /= 1000
	}
	switch {
	case strings.Contains(format, "flac") && bitrate >= 1000:
		return model.AudioQualityHiRes
	case strings.Contains(format, "flac"):
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
