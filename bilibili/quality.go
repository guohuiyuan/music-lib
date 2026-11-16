package bilibili

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	bilibiliQualityProbeBVID = "BV1zYWB6qED9"
	bilibiliQualityProbeCID  = "43575806705"
)

type bilibiliDashAudio struct {
	ID      int    `json:"id"`
	BaseURL string `json:"baseUrl"`
}

type bilibiliPlayURLResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Durl []struct {
			URL string `json:"url"`
		} `json:"durl"`
		Dash struct {
			Audio []bilibiliDashAudio `json:"audio"`
			Flac  struct {
				Audio bilibiliDashAudio `json:"audio"`
			} `json:"flac"`
			Dolby struct {
				Audio []bilibiliDashAudio `json:"audio"`
			} `json:"dolby"`
		} `json:"dash"`
	} `json:"data"`
}

type bilibiliAudioCandidate struct {
	option model.AudioQualityOption
	url    string
	id     int
	rank   int
}

type bilibiliQualityDefinition struct {
	option model.AudioQualityOption
	rank   int
}

var bilibiliQualityDefinitions = map[model.AudioQuality]bilibiliQualityDefinition{
	model.AudioQualityHiRes: {
		option: model.AudioQualityOption{
			Value:  model.AudioQualityHiRes,
			Label:  "Hi-Res 无损",
			Format: "flac",
			VIP:    true,
		},
		rank: 5,
	},
	model.AudioQualityAtmos: {
		option: model.AudioQualityOption{
			Value:  model.AudioQualityAtmos,
			Label:  "全景声",
			Format: "m4a",
			VIP:    true,
		},
		rank: 4,
	},
	model.AudioQualityHigh: {
		option: model.AudioQualityOption{
			Value:   model.AudioQualityHigh,
			Label:   "高品质",
			Format:  "m4a",
			Bitrate: 192,
		},
		rank: 3,
	},
	model.AudioQualityStandard: {
		option: model.AudioQualityOption{
			Value:   model.AudioQualityStandard,
			Label:   "标准音质",
			Format:  "m4a",
			Bitrate: 132,
		},
		rank: 2,
	},
	model.AudioQualityLow: {
		option: model.AudioQualityOption{
			Value:   model.AudioQualityLow,
			Label:   "低品质",
			Format:  "m4a",
			Bitrate: 64,
		},
		rank: 1,
	},
}

var bilibiliQualityOrder = []model.AudioQuality{
	model.AudioQualityHiRes,
	model.AudioQualityAtmos,
	model.AudioQualityHigh,
	model.AudioQualityStandard,
	model.AudioQualityLow,
}

// ListQualityOptions returns every Bilibili audio tier recognized by the
// DASH playurl API. The existing Hi-Res-first priority is preserved.
func ListQualityOptions() []model.AudioQualityOption {
	options := make([]model.AudioQualityOption, 0, len(bilibiliQualityOrder))
	for _, quality := range bilibiliQualityOrder {
		options = append(options, bilibiliQualityDefinitions[quality].option)
	}
	return options
}

// AvailableQualityOptions probes a public music video using the configured
// cookie. VIP-only tracks appear only when the playurl response exposes them.
func (b *Bilibili) AvailableQualityOptions() ([]model.AudioQualityOption, error) {
	isVip, _ := b.IsVipAccount()
	candidates, err := b.fetchAudioCandidates(
		bilibiliQualityProbeBVID,
		bilibiliQualityProbeCID,
		isVip,
	)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, errors.New("bilibili returned no playable audio")
	}

	seen := make(map[model.AudioQuality]bool)
	options := make([]model.AudioQualityOption, 0, len(candidates))
	for _, candidate := range candidates {
		if seen[candidate.option.Value] {
			continue
		}
		seen[candidate.option.Value] = true
		options = append(options, candidate.option)
	}
	return options, nil
}

// GetDownloadURLWithQuality resolves an exact Bilibili DASH audio track. Auto
// keeps the previous behavior: Hi-Res, then Dolby, then the largest normal ID.
func (b *Bilibili) GetDownloadURLWithQuality(song *model.Song, quality string) (string, error) {
	normalized, ok := model.NormalizeAudioQuality(quality)
	if !ok {
		return "", fmt.Errorf("unsupported bilibili audio quality: %s", quality)
	}
	if song == nil {
		return "", errors.New("song is nil")
	}
	if !strings.EqualFold(strings.TrimSpace(song.Source), "bilibili") {
		return "", errors.New("source mismatch")
	}
	if normalized == model.AudioQualityAuto {
		return b.GetDownloadURL(song)
	}
	if _, ok := bilibiliQualityDefinitions[normalized]; !ok {
		return "", fmt.Errorf("unsupported bilibili audio quality: %s", quality)
	}

	bvid, cid, err := bilibiliSongIdentity(song)
	if err != nil {
		return "", err
	}
	isVip, _ := b.IsVipAccount()
	candidates, err := b.fetchAudioCandidates(bvid, cid, isVip)
	if err != nil {
		return "", err
	}
	for _, candidate := range candidates {
		if candidate.option.Value == normalized {
			return candidate.url, nil
		}
	}
	return "", fmt.Errorf("bilibili audio quality %s is unavailable", normalized)
}

func (b *Bilibili) fetchAudioCandidates(
	bvid string,
	cid string,
	isVip bool,
) ([]bilibiliAudioCandidate, error) {
	bvid = strings.TrimSpace(bvid)
	cid = strings.TrimSpace(cid)
	if bvid == "" || cid == "" {
		return nil, errors.New("bilibili bvid or cid is empty")
	}

	fnval := 80
	if isVip {
		fnval = 4048
	}
	apiURL := fmt.Sprintf(
		"https://api.bilibili.com/x/player/playurl?fnval=%d&qn=127&bvid=%s&cid=%s",
		fnval,
		bvid,
		cid,
	)
	body, err := utils.Get(
		apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Cookie", b.cookie),
	)
	if err != nil {
		return nil, err
	}

	var resp bilibiliPlayURLResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("bilibili playurl json error: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("bilibili playurl api error: %d %s", resp.Code, resp.Message)
	}

	candidates := make([]bilibiliAudioCandidate, 0, 5)
	candidates = appendBilibiliAudioCandidate(candidates, resp.Data.Dash.Flac.Audio)
	for _, audio := range resp.Data.Dash.Dolby.Audio {
		candidates = appendBilibiliAudioCandidate(candidates, audio)
	}
	for _, audio := range resp.Data.Dash.Audio {
		candidates = appendBilibiliAudioCandidate(candidates, audio)
	}
	if len(candidates) == 0 {
		for _, durl := range resp.Data.Durl {
			if strings.TrimSpace(durl.URL) == "" {
				continue
			}
			definition := bilibiliQualityDefinitions[model.AudioQualityStandard]
			candidates = append(candidates, bilibiliAudioCandidate{
				option: definition.option,
				url:    strings.TrimSpace(durl.URL),
				rank:   definition.rank,
			})
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].rank != candidates[j].rank {
			return candidates[i].rank > candidates[j].rank
		}
		return candidates[i].id > candidates[j].id
	})
	return candidates, nil
}

func appendBilibiliAudioCandidate(
	candidates []bilibiliAudioCandidate,
	audio bilibiliDashAudio,
) []bilibiliAudioCandidate {
	audio.BaseURL = strings.TrimSpace(audio.BaseURL)
	if audio.BaseURL == "" {
		return candidates
	}
	quality, ok := bilibiliQualityForID(audio.ID)
	if !ok {
		return candidates
	}
	definition := bilibiliQualityDefinitions[quality]
	return append(candidates, bilibiliAudioCandidate{
		option: definition.option,
		url:    audio.BaseURL,
		id:     audio.ID,
		rank:   definition.rank,
	})
}

func bilibiliQualityForID(id int) (model.AudioQuality, bool) {
	switch id {
	case 30251:
		return model.AudioQualityHiRes, true
	case 30250:
		return model.AudioQualityAtmos, true
	case 30280:
		return model.AudioQualityHigh, true
	case 30232:
		return model.AudioQualityStandard, true
	case 30216:
		return model.AudioQualityLow, true
	default:
		return "", false
	}
}

func bilibiliSongIdentity(song *model.Song) (string, string, error) {
	if song == nil {
		return "", "", errors.New("song is nil")
	}

	var bvid, cid string
	if song.Extra != nil {
		bvid = strings.TrimSpace(song.Extra["bvid"])
		cid = strings.TrimSpace(song.Extra["cid"])
	}
	if bvid == "" || cid == "" {
		parts := strings.Split(song.ID, "|")
		if len(parts) != 2 {
			return "", "", errors.New("invalid id structure")
		}
		if bvid == "" {
			bvid = strings.TrimSpace(parts[0])
		}
		if cid == "" {
			cid = strings.TrimSpace(parts[1])
		}
	}
	if bvid == "" || cid == "" {
		return "", "", errors.New("invalid id structure")
	}
	return bvid, cid, nil
}
