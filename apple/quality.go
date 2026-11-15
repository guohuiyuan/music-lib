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

// ListQualityOptions returns every playable Apple quality tier.
func ListQualityOptions() []model.AudioQualityOption {
	return []model.AudioQualityOption{applePreviewQuality}
}

// AvailableQualityOptions reports the only stream Apple currently exposes
// without DRM decryption: its official preview.
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
