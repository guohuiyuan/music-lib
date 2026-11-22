package jamendo

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
)

const jamendoQualityProbeKeyword = "love"

type jamendoQualityCandidate struct {
	option  model.AudioQualityOption
	sources []jamendoQualitySource
}

type jamendoQualitySource struct {
	field  string
	stream bool
}

var jamendoQualityCandidates = []jamendoQualityCandidate{
	{
		option: model.AudioQualityOption{
			Value:  model.AudioQualityLossless,
			Label:  "无损音质",
			Format: "flac",
		},
		sources: []jamendoQualitySource{
			{field: "flac"},
			{field: "flac", stream: true},
		},
	},
	{
		option: model.AudioQualityOption{
			Value:   model.AudioQualityHigh,
			Label:   "高品质",
			Format:  "mp3",
			Bitrate: 320,
		},
		sources: []jamendoQualitySource{
			{field: "mp33"},
			{field: "mp33", stream: true},
		},
	},
	{
		option: model.AudioQualityOption{
			Value:   model.AudioQualityHigher,
			Label:   "较高品质",
			Format:  "mp3",
			Bitrate: 192,
		},
		sources: []jamendoQualitySource{
			{field: "mp32"},
			{field: "mp32", stream: true},
			{field: "mp3"},
		},
	},
	{
		option: model.AudioQualityOption{
			Value:   model.AudioQualityStandard,
			Label:   "标准音质",
			Format:  "ogg",
			Bitrate: 112,
		},
		sources: []jamendoQualitySource{
			{field: "ogg"},
			{field: "ogg", stream: true},
		},
	},
	{
		option: model.AudioQualityOption{
			Value:   model.AudioQualityLow,
			Label:   "低品质",
			Format:  "mp3",
			Bitrate: 96,
		},
		sources: []jamendoQualitySource{
			{field: "mp3", stream: true},
		},
	},
}

var jamendoQualityProbeClient = &http.Client{Timeout: 20 * time.Second}

// ListQualityOptions 返回 Jamendo 所有可下载的音频档位。
func ListQualityOptions() []model.AudioQualityOption {
	options := make([]model.AudioQualityOption, 0, len(jamendoQualityCandidates))
	for _, candidate := range jamendoQualityCandidates {
		options = append(options, candidate.option)
	}
	return options
}

// AvailableQualityOptions 会校验音频流地址，因为 Jamendo 可能返回 mp33 地址，
// 而存储后端对该歌曲实际返回 404。
func (j *Jamendo) AvailableQualityOptions() ([]model.AudioQualityOption, error) {
	items, err := j.jamendoQualityProbeItems()
	if err != nil {
		return nil, err
	}

	options := make([]model.AudioQualityOption, 0, len(jamendoQualityCandidates))
	for _, candidate := range jamendoQualityCandidates {
		available := false
		for _, item := range items {
			streamURL := jamendoQualityURL(item, candidate)
			if streamURL == "" {
				continue
			}
			if err := jamendoProbeStream(streamURL); err == nil {
				available = true
				break
			}
		}
		if available {
			options = append(options, candidate.option)
		}
	}
	if len(options) == 0 {
		return nil, errors.New("jamendo returned no playable quality")
	}
	return options, nil
}

// GetDownloadURLWithQuality 解析指定的 Jamendo 档位。自动模式保持原有的
// “最佳可下载音频流”行为。
func (j *Jamendo) GetDownloadURLWithQuality(song *model.Song, quality string) (string, error) {
	normalized, ok := model.NormalizeAudioQuality(quality)
	if !ok {
		return "", fmt.Errorf("unsupported jamendo audio quality: %s", quality)
	}
	if song == nil {
		return "", errors.New("song is nil")
	}
	if !strings.EqualFold(strings.TrimSpace(song.Source), "jamendo") {
		return "", errors.New("source mismatch")
	}
	if normalized == model.AudioQualityAuto {
		return j.GetDownloadURL(song)
	}

	candidate, ok := jamendoQualityCandidateFor(normalized)
	if !ok {
		return "", fmt.Errorf("unsupported jamendo audio quality: %s", quality)
	}
	trackID := strings.TrimSpace(song.ID)
	if song.Extra != nil && strings.TrimSpace(song.Extra["track_id"]) != "" {
		trackID = strings.TrimSpace(song.Extra["track_id"])
	}
	if trackID == "" {
		return "", errors.New("jamendo track id is empty")
	}

	item, err := j.fetchTrackItem(trackID)
	if err != nil {
		return "", err
	}
	streamURL := jamendoQualityURL(item, candidate)
	if streamURL == "" {
		return "", fmt.Errorf("jamendo audio quality %s is unavailable", normalized)
	}
	if err := jamendoProbeStream(streamURL); err != nil {
		return "", fmt.Errorf("jamendo audio quality %s is unavailable: %w", normalized, err)
	}
	return streamURL, nil
}

func (j *Jamendo) jamendoQualityProbeItems() ([]jamendoTrackItem, error) {
	body, err := j.searchByType(jamendoQualityProbeKeyword, "track")
	if err != nil {
		return nil, err
	}

	var results []jamendoTrackItem
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("jamendo quality probe json error: %w", err)
	}
	if len(results) == 0 {
		return nil, errors.New("jamendo quality probe found no tracks")
	}
	if len(results) > 8 {
		results = results[:8]
	}
	return results, nil
}

func jamendoQualityCandidateFor(quality model.AudioQuality) (jamendoQualityCandidate, bool) {
	for _, candidate := range jamendoQualityCandidates {
		if candidate.option.Value == quality {
			return candidate, true
		}
	}
	return jamendoQualityCandidate{}, false
}

func jamendoQualityURL(item jamendoTrackItem, candidate jamendoQualityCandidate) string {
	for _, source := range candidate.sources {
		streams := item.Download
		if source.stream {
			streams = item.Stream
		}
		if streamURL := strings.TrimSpace(streams[source.field]); streamURL != "" {
			return streamURL
		}
	}
	return ""
}

func jamendoProbeStream(streamURL string) error {
	req, err := http.NewRequest(http.MethodHead, streamURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Referer", Referer)

	resp, err := jamendoQualityProbeClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("http request failed: status %d", resp.StatusCode)
	}
	return nil
}
