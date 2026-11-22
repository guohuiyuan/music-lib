package apple

import (
	"errors"
	"fmt"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
)

const appleQualityProbeSongID = "255921025"

var applePreviewQuality = model.AudioQualityOption{
	Value:  model.AudioQualityPreview,
	Label:  "试听片段",
	Format: "m4a",
}

// ListQualityOptions 返回 Apple 所有可播放的音质档位。
func ListQualityOptions() []model.AudioQualityOption {
	return []model.AudioQualityOption{applePreviewQuality}
}

// AvailableQualityOptions 报告 Apple 目前唯一无需 DRM 解密就能获取的音频流：
// 官方试听片段。
func (a *Apple) AvailableQualityOptions() ([]model.AudioQualityOption, error) {
	_, err := a.GetDownloadURL(&model.Song{Source: "apple", ID: appleQualityProbeSongID})
	if err != nil {
		return nil, err
	}
	return []model.AudioQualityOption{applePreviewQuality}, nil
}

func (a *Apple) GetDownloadURLWithQuality(song *model.Song, quality string) (string, error) {
	normalized, ok := model.NormalizeAudioQuality(quality)
	if !ok {
		return "", fmt.Errorf("unsupported apple audio quality: %s", quality)
	}
	if normalized != model.AudioQualityAuto && normalized != model.AudioQualityPreview {
		return "", errors.New("apple full-quality streams require DRM support; only preview is available")
	}
	if song == nil {
		return "", errors.New("song is nil")
	}
	if song.Source != "" && !strings.EqualFold(song.Source, "apple") {
		return "", errors.New("source mismatch")
	}
	return a.GetDownloadURL(song)
}
