package qq

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const qqQualityProbeSongMID = "0039MnYb0qxYhV"

type qualityCandidate struct {
	prefix  string
	ext     string
	quality model.AudioQualityOption
}

// 这里的顺序同时也是自动选择优先级：音质从高到低。
var qqQualityCandidates = []qualityCandidate{
	{
		prefix: "AI00",
		ext:    "flac",
		quality: model.AudioQualityOption{
			Value: model.AudioQualityMaster, Label: "臻品母带", Format: "flac", VIP: true,
		},
	},
	{
		prefix: "Q001",
		ext:    "flac",
		quality: model.AudioQualityOption{
			Value: model.AudioQualityAtmos51, Label: "全景声 5.1", Format: "flac", VIP: true,
		},
	},
	{
		prefix: "Q000",
		ext:    "flac",
		quality: model.AudioQualityOption{
			Value: model.AudioQualityAtmos, Label: "全景声 2.0", Format: "flac", VIP: true,
		},
	},
	{
		prefix: "F000",
		ext:    "flac",
		quality: model.AudioQualityOption{
			Value: model.AudioQualityLossless, Label: "无损音质", Format: "flac", VIP: true,
		},
	},
	{
		prefix: "O801",
		ext:    "ogg",
		quality: model.AudioQualityOption{
			Value: model.AudioQualityUltra, Label: "超高品质", Format: "ogg", Bitrate: 640, VIP: true,
		},
	},
	{
		prefix: "M800",
		ext:    "mp3",
		quality: model.AudioQualityOption{
			Value: model.AudioQualityHigh, Label: "高品质", Format: "mp3", Bitrate: 320,
		},
	},
	{
		prefix: "M500",
		ext:    "mp3",
		quality: model.AudioQualityOption{
			Value: model.AudioQualityStandard, Label: "标准音质", Format: "mp3", Bitrate: 128,
		},
	},
}

// ListQualityOptions 返回 QQ 音乐支持的全部音质档位。
func ListQualityOptions() []model.AudioQualityOption {
	options := make([]model.AudioQualityOption, 0, len(qqQualityCandidates))
	for _, candidate := range qqQualityCandidates {
		options = append(options, candidate.quality)
	}
	return options
}

// AvailableQualityOptions 探测当前账号并返回实际能选到的档位。只有探测返回
// 可播放地址时才会包含该档位，因为匿名可拿到的元数据不代表匿名可播放。
func (q *QQ) AvailableQualityOptions() ([]model.AudioQualityOption, error) {
	if q.availableQualityCache != nil {
		return append([]model.AudioQualityOption(nil), q.availableQualityCache...), nil
	}

	purls, err := q.requestQualityURLs(qqQualityProbeSongMID, qqQualityCandidates)
	if err != nil {
		isVip := false
		q.isVipCache = &isVip
		if strings.TrimSpace(q.cookie) == "" {
			// QQ 目前对匿名 vkey 请求返回 API code 1000。
			// 元数据搜索仍然可用，但没有任何可播放的音频档位。
			q.availableQualityCache = []model.AudioQualityOption{}
			return []model.AudioQualityOption{}, nil
		}
		return nil, err
	}

	options := availableQualityOptionsFromPurls(qqQualityProbeSongMID, purls)
	q.availableQualityCache = append([]model.AudioQualityOption(nil), options...)
	isVip := false
	for _, option := range options {
		if option.VIP {
			isVip = true
			break
		}
	}
	q.isVipCache = &isVip
	return append([]model.AudioQualityOption(nil), options...), nil
}

func availableQualityOptionsFromPurls(songMID string, purls map[string]string) []model.AudioQualityOption {
	options := make([]model.AudioQualityOption, 0, len(qqQualityCandidates))
	for _, candidate := range qqQualityCandidates {
		if purls[qualityFilename(songMID, candidate)] == "" {
			continue
		}
		options = append(options, candidate.quality)
	}
	return options
}

func qualityCandidateFor(value string) (qualityCandidate, bool) {
	quality, ok := model.NormalizeAudioQuality(value)
	if !ok || quality == model.AudioQualityAuto {
		return qualityCandidate{}, false
	}
	for _, candidate := range qqQualityCandidates {
		if candidate.quality.Value == quality {
			return candidate, true
		}
	}
	return qualityCandidate{}, false
}

func qualityFilename(songMID string, candidate qualityCandidate) string {
	return fmt.Sprintf("%s%s%s.%s", candidate.prefix, songMID, songMID, candidate.ext)
}

func (q *QQ) requestQualityURLs(songMID string, candidates []qualityCandidate) (map[string]string, error) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	guid := fmt.Sprintf("%d", r.Int63n(9000000000)+1000000000)

	filenames := make([]string, 0, len(candidates))
	songmids := make([]string, 0, len(candidates))
	songtypes := make([]int, 0, len(candidates))
	for _, candidate := range candidates {
		filenames = append(filenames, qualityFilename(songMID, candidate))
		songmids = append(songmids, songMID)
		songtypes = append(songtypes, 0)
	}

	reqData := map[string]interface{}{
		"comm": map[string]interface{}{
			"cv":          4747474,
			"ct":          24,
			"format":      "json",
			"inCharset":   "utf-8",
			"outCharset":  "utf-8",
			"notice":      0,
			"platform":    "yqq.json",
			"needNewCode": 1,
			"uin":         0,
		},
		"req_1": map[string]interface{}{
			"module": "music.vkey.GetVkey",
			"method": "UrlGetVkey",
			"param": map[string]interface{}{
				"guid":      guid,
				"songmid":   songmids,
				"songtype":  songtypes,
				"uin":       "0",
				"loginflag": 1,
				"platform":  "20",
				"filename":  filenames,
			},
		},
	}

	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return nil, fmt.Errorf("qq geturl request encode error: %w", err)
	}
	headers := []utils.RequestOption{
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", DownloadReferer),
		utils.WithHeader("Content-Type", "application/json"),
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	}

	body, err := utils.Post("https://u.y.qq.com/cgi-bin/musicu.fcg", bytes.NewReader(jsonData), headers...)
	if err != nil {
		return nil, err
	}

	var result struct {
		Req1 struct {
			Code int `json:"code"`
			Data struct {
				MidUrlInfo []struct {
					Filename string `json:"filename"`
					Purl     string `json:"purl"`
				} `json:"midurlinfo"`
			} `json:"data"`
		} `json:"req_1"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("qq geturl json parse error: %w", err)
	}
	if result.Req1.Code != 0 {
		return nil, fmt.Errorf("qq geturl api error code: %d", result.Req1.Code)
	}

	purls := make(map[string]string, len(result.Req1.Data.MidUrlInfo))
	for _, info := range result.Req1.Data.MidUrlInfo {
		if info.Filename != "" && info.Purl != "" {
			purls[info.Filename] = info.Purl
		}
	}
	return purls, nil
}

// GetDownloadURLWithQuality 返回指定音质档位的地址。音质为空或为 auto 时
// 走兼容逻辑，选择当前账号能拿到的最佳地址。
func (q *QQ) GetDownloadURLWithQuality(s *model.Song, quality string) (string, error) {
	if s == nil {
		return "", errors.New("song is nil")
	}
	if s.Source != "qq" {
		return "", errors.New("source mismatch")
	}

	songMID := s.ID
	if s.Extra != nil && strings.TrimSpace(s.Extra["songmid"]) != "" {
		songMID = s.Extra["songmid"]
	}
	if strings.TrimSpace(songMID) == "" {
		return "", errors.New("qq song id is empty")
	}

	normalized, ok := model.NormalizeAudioQuality(quality)
	if !ok {
		return "", fmt.Errorf("unsupported qq audio quality: %s", quality)
	}

	candidates := qqQualityCandidates
	if normalized != model.AudioQualityAuto {
		candidate, exists := qualityCandidateFor(string(normalized))
		if !exists {
			return "", fmt.Errorf("unsupported qq audio quality: %s", quality)
		}
		candidates = []qualityCandidate{candidate}
	}

	purls, err := q.requestQualityURLs(songMID, candidates)
	if err != nil {
		return "", err
	}
	for _, candidate := range candidates {
		if purl := purls[qualityFilename(songMID, candidate)]; purl != "" {
			return "https://ws.stream.qqmusic.qq.com/" + purl, nil
		}
	}
	if normalized == model.AudioQualityAuto {
		return "", errors.New("no valid download url found or vip required")
	}
	return "", fmt.Errorf("qq audio quality %s is unavailable for this song or account", normalized)
}
