package soda

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

var sodaQualityCandidates = []model.AudioQualityOption{
	{Value: model.AudioQualitySpatial, Label: "空间音频", Format: "m4a", VIP: true},
	{Value: model.AudioQualityHiRes, Label: "Hi-Res 无损", Format: "flac", VIP: true},
	{Value: model.AudioQualityLossless, Label: "无损音质", Format: "flac", VIP: true},
	{Value: model.AudioQualityHigh, Label: "高品质 320k", Format: "m4a", Bitrate: 320},
	{Value: model.AudioQualityHigher, Label: "较高品质 128k", Format: "m4a", Bitrate: 128},
	{Value: model.AudioQualityStandard, Label: "标准音质 64k", Format: "m4a", Bitrate: 64},
	{Value: model.AudioQualityPreview, Label: "试听片段", Format: "m4a"},
}

// ListQualityOptions returns the complete Soda quality taxonomy.
func ListQualityOptions() []model.AudioQualityOption {
	return append([]model.AudioQualityOption(nil), sodaQualityCandidates...)
}

// AvailableQualityOptions probes real streams for a representative Soda track.
func (s *Soda) AvailableQualityOptions() ([]model.AudioQualityOption, error) {
	if s.availableQualityCache != nil {
		return append([]model.AudioQualityOption(nil), s.availableQualityCache...), nil
	}

	streams, err := s.collectAvailableStreams(vipProbeTrackID)
	if err != nil {
		return nil, err
	}
	seen := make(map[model.AudioQuality]struct{}, len(streams))
	options := make([]model.AudioQualityOption, 0, len(sodaQualityCandidates))
	for _, option := range sodaQualityCandidates {
		for _, info := range streams {
			if sodaDownloadInfoQuality(info) != option.Value {
				continue
			}
			seen[option.Value] = struct{}{}
			options = append(options, option)
			break
		}
	}
	if len(options) == 0 {
		return nil, errors.New("soda returned no playable quality")
	}
	s.availableQualityCache = append([]model.AudioQualityOption(nil), options...)
	return append([]model.AudioQualityOption(nil), options...), nil
}

func (s *Soda) GetDownloadInfoWithQuality(song *model.Song, quality string) (*DownloadInfo, error) {
	normalized, ok := model.NormalizeAudioQuality(quality)
	if !ok {
		return nil, fmt.Errorf("unsupported soda audio quality: %s", quality)
	}
	if normalized == model.AudioQualityAuto {
		return s.GetDownloadInfo(song)
	}
	if song == nil {
		return nil, errors.New("song is nil")
	}
	if song.Source != "" && song.Source != "soda" {
		return nil, errors.New("source mismatch")
	}
	trackID := sodaSongTrackID(song)
	if trackID == "" {
		return nil, errors.New("track id is empty")
	}

	streams, err := s.collectAvailableStreams(trackID)
	if err != nil {
		cached := sodaCachedDownloadInfo(song)
		if cached != nil && sodaDownloadInfoQuality(cached) == normalized {
			return cached, nil
		}
		return nil, err
	}
	best := selectSodaQualityStream(streams, normalized)
	if best == nil {
		return nil, fmt.Errorf("soda audio quality %s is unavailable for this song or account", normalized)
	}
	return best, nil
}

func (s *Soda) GetDownloadURLWithQuality(song *model.Song, quality string) (string, error) {
	info, err := s.GetDownloadInfoWithQuality(song, quality)
	if err != nil {
		return "", err
	}
	return sodaDownloadInfoURL(info), nil
}

func (s *Soda) collectAvailableStreams(trackID string) ([]*DownloadInfo, error) {
	webResp, err := s.fetchWebTrackV2(trackID)
	if err != nil {
		return nil, err
	}
	fullDuration := sodaTrackDurationSeconds(webResp.primaryTrack())
	streams := sodaDownloadInfosFromVideoModel(webResp.TrackPlayer.VideoModel, fullDuration)
	if webResp.TrackPlayer.URLPlayerInfo != "" {
		playerStreams, playerErr := s.fetchPlayerInfoList(webResp.TrackPlayer.URLPlayerInfo, fullDuration)
		if playerErr == nil {
			streams = append(streams, playerStreams...)
		} else if len(streams) == 0 {
			return nil, playerErr
		}
	}

	if s.CanFetchSignedStream() && webResp.primaryTrack().LabelInfo.IsVIP() {
		if pcResp, pcErr := s.fetchPCTrackV2(trackID); pcErr == nil {
			pcDuration := sodaTrackDurationSeconds(pcResp.primaryTrack())
			if pcDuration > 0 {
				fullDuration = pcDuration
			}
			streams = append(streams, sodaDownloadInfosFromVideoModel(pcResp.TrackPlayer.VideoModel, fullDuration)...)
			if pcResp.TrackPlayer.URLPlayerInfo != "" {
				playerStreams, playerErr := s.fetchPlayerInfoList(pcResp.TrackPlayer.URLPlayerInfo, fullDuration)
				if playerErr == nil {
					streams = append(streams, playerStreams...)
				}
			}
		}
	}

	filtered := make([]*DownloadInfo, 0, len(streams))
	seen := make(map[string]struct{}, len(streams))
	for _, info := range streams {
		if info == nil || strings.TrimSpace(info.URL) == "" {
			continue
		}
		key := info.URL + "\x00" + info.PlayAuth
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		filtered = append(filtered, info)
	}
	if len(filtered) == 0 {
		return nil, errors.New("soda player info not found")
	}
	return filtered, nil
}

func sodaDownloadInfosFromVideoModel(raw []byte, fullDuration int) []*DownloadInfo {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return nil
	}
	for i := 0; i < 3 && strings.HasPrefix(text, "\""); i++ {
		var nested string
		if err := json.Unmarshal([]byte(text), &nested); err != nil {
			break
		}
		text = strings.TrimSpace(nested)
	}

	var value any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		return nil
	}
	entries := make([]sodaVideoModelEntry, 0)
	sodaCollectVideoModelEntries(value, "", "", 0, &entries)

	result := make([]*DownloadInfo, 0, len(entries))
	for _, entry := range entries {
		downloadURL := strings.TrimSpace(entry.MainPlayURL)
		if downloadURL == "" {
			downloadURL = strings.TrimSpace(entry.BackupPlayURL)
		}
		if downloadURL == "" {
			continue
		}
		info := &DownloadInfo{
			URL:      downloadURL,
			PlayAuth: strings.TrimSpace(entry.PlayAuth),
			Format:   strings.TrimSpace(entry.Format),
			Size:     entry.Size,
			Duration: entry.Duration,
			Bitrate:  entry.Bitrate,
			Quality:  strings.TrimSpace(entry.Quality),
		}
		info.Preview = sodaDownloadInfoIsPreview(info, fullDuration)
		result = append(result, info)
	}
	return result
}

func (s *Soda) fetchPlayerInfoList(playerInfoURL string, fullDuration int) ([]*DownloadInfo, error) {
	infoBody, err := utils.Get(playerInfoURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", s.cookie),
	)
	if err != nil {
		return nil, err
	}
	var infoResp struct {
		ResponseMetadata struct {
			Error struct {
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"ResponseMetadata"`
		Result struct {
			Data struct {
				PlayInfoList []sodaPlayerInfo `json:"PlayInfoList"`
			} `json:"Data"`
		} `json:"Result"`
	}
	if err := json.Unmarshal(infoBody, &infoResp); err != nil {
		return nil, fmt.Errorf("parse play info response error: %w", err)
	}
	if len(infoResp.Result.Data.PlayInfoList) == 0 {
		if infoResp.ResponseMetadata.Error.Message != "" {
			return nil, errors.New(infoResp.ResponseMetadata.Error.Message)
		}
		return nil, errors.New("no audio stream found")
	}

	result := make([]*DownloadInfo, 0, len(infoResp.Result.Data.PlayInfoList))
	for _, stream := range infoResp.Result.Data.PlayInfoList {
		downloadURL := strings.TrimSpace(stream.MainPlayURL)
		if downloadURL == "" {
			downloadURL = strings.TrimSpace(stream.BackupPlayURL)
		}
		if downloadURL == "" {
			continue
		}
		info := &DownloadInfo{
			URL:      downloadURL,
			PlayAuth: strings.TrimSpace(stream.PlayAuth),
			Format:   strings.TrimSpace(stream.Format),
			Size:     stream.Size,
			Duration: stream.Duration,
			Bitrate:  stream.Bitrate,
			Quality:  stream.Quality,
		}
		info.Preview = sodaDownloadInfoIsPreview(info, fullDuration)
		result = append(result, info)
	}
	if len(result) == 0 {
		return nil, errors.New("no audio stream found")
	}
	return result, nil
}

func sodaDownloadInfoQuality(info *DownloadInfo) model.AudioQuality {
	if info == nil {
		return ""
	}
	if info.Preview {
		return model.AudioQualityPreview
	}
	q := strings.ToLower(strings.TrimSpace(info.Quality))
	q = strings.NewReplacer("-", "", "_", "", " ", "").Replace(q)
	format := strings.ToLower(strings.TrimSpace(info.Format))
	bitrate := normalizeSodaBitrate(info.Bitrate)
	rank := sodaQualityRank(info.Quality, info.Format, info.Bitrate)

	switch {
	case strings.Contains(q, "spatial"), strings.Contains(q, "atmos"), strings.Contains(q, "dolby"):
		return model.AudioQualitySpatial
	case rank >= 110:
		return model.AudioQualityHiRes
	case rank >= 100:
		return model.AudioQualityLossless
	case strings.Contains(format, "flac"), strings.Contains(format, "alac"), strings.Contains(format, "wav"):
		return model.AudioQualityLossless
	case rank >= 80, bitrate >= 280:
		return model.AudioQualityHigh
	case rank >= 70, bitrate >= 192:
		return model.AudioQualityHigher
	case rank >= 50, bitrate >= 96:
		return model.AudioQualityStandard
	default:
		return model.AudioQualityLow
	}
}

func selectSodaQualityStream(streams []*DownloadInfo, quality model.AudioQuality) *DownloadInfo {
	var best *DownloadInfo
	for _, info := range streams {
		if sodaDownloadInfoQuality(info) != quality {
			continue
		}
		if best == nil {
			best = info
			continue
		}
		if best.Preview && !info.Preview {
			best = info
			continue
		}
		if sodaBetterStreamCandidate(
			info.Duration, info.Quality, info.Format, info.Bitrate, info.Size,
			best.Duration, best.Quality, best.Format, best.Bitrate, best.Size,
		) {
			best = info
		}
	}
	return best
}
