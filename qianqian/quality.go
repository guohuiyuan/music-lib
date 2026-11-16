package qianqian

import (
	"errors"
	"fmt"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
)

const qianqianQualityProbeKeyword = "痛仰乐队"

type qianqianQualityCandidate struct {
	option model.AudioQualityOption
	rate   string
}

var qianqianQualityCandidates = []qianqianQualityCandidate{
	{
		option: model.AudioQualityOption{Value: model.AudioQualityLossless, Label: "无损音质", Format: "flac"},
		rate:   "3000",
	},
	{
		option: model.AudioQualityOption{Value: model.AudioQualityHigh, Label: "高品质 320k", Format: "mp3", Bitrate: 320},
		rate:   "320",
	},
	{
		option: model.AudioQualityOption{Value: model.AudioQualityStandard, Label: "标准音质 128k", Format: "mp3", Bitrate: 128},
		rate:   "128",
	},
	{
		option: model.AudioQualityOption{Value: model.AudioQualityLow, Label: "低品质 64k", Format: "mp3", Bitrate: 64},
		rate:   "64",
	},
}

func ListQualityOptions() []model.AudioQualityOption {
	options := make([]model.AudioQualityOption, 0, len(qianqianQualityCandidates))
	for _, candidate := range qianqianQualityCandidates {
		options = append(options, candidate.option)
	}
	return options
}

func (q *Qianqian) AvailableQualityOptions() ([]model.AudioQualityOption, error) {
	if q.availableQualityCache != nil {
		return append([]model.AudioQualityOption(nil), q.availableQualityCache...), nil
	}

	songs, err := q.Search(qianqianQualityProbeKeyword)
	if err != nil {
		return nil, err
	}
	options := make([]model.AudioQualityOption, 0, len(qianqianQualityCandidates))
	for _, candidate := range qianqianQualityCandidates {
		for i := range songs {
			if i >= 5 {
				break
			}
			if _, err := q.GetDownloadURLWithQuality(&songs[i], string(candidate.option.Value)); err == nil {
				options = append(options, candidate.option)
				break
			}
		}
	}
	q.availableQualityCache = append([]model.AudioQualityOption(nil), options...)
	return append([]model.AudioQualityOption(nil), options...), nil
}

func (q *Qianqian) GetDownloadURLWithQuality(song *model.Song, quality string) (string, error) {
	normalized, ok := model.NormalizeAudioQuality(quality)
	if !ok {
		return "", fmt.Errorf("unsupported qianqian audio quality: %s", quality)
	}
	if normalized == model.AudioQualityAuto {
		return q.GetDownloadURL(song)
	}
	if song == nil {
		return "", errors.New("song is nil")
	}
	if song.Source != "" && !strings.EqualFold(song.Source, "qianqian") {
		return "", errors.New("source mismatch")
	}
	candidate, ok := qianqianQualityCandidateFor(normalized)
	if !ok {
		return "", fmt.Errorf("unsupported qianqian audio quality: %s", quality)
	}
	tsid := strings.TrimSpace(song.ID)
	if song.Extra != nil {
		tsid = strings.TrimSpace(firstNonEmpty(song.Extra["tsid"], tsid))
	}
	if tsid == "" {
		return "", errors.New("qianqian tsid is empty")
	}
	return q.fetchDownloadURLWithRate(tsid, candidate.rate)
}

func qianqianQualityCandidateFor(quality model.AudioQuality) (qianqianQualityCandidate, bool) {
	for _, candidate := range qianqianQualityCandidates {
		if candidate.option.Value == quality {
			return candidate, true
		}
	}
	return qianqianQualityCandidate{}, false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
