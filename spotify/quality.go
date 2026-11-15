package spotify

import (
	"errors"
	"fmt"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
)

const spotifyQualityProbeSongID = "2sFepQNyl1WjpROruP8bJ0"

var spotifyPreviewQuality = model.AudioQualityOption{
	Value:  model.AudioQualityPreview,
	Label:  "试听片段",
	Format: "m4a",
}

// ListQualityOptions returns every playable Spotify quality tier.
func ListQualityOptions() []model.AudioQualityOption {
	return []model.AudioQualityOption{spotifyPreviewQuality}
}

// AvailableQualityOptions reports the playable preview exposed by Spotify
// without handling DRM-protected full tracks.
func (s *Spotify) AvailableQualityOptions() ([]model.AudioQualityOption, error) {
	_, err := s.GetDownloadURL(&model.Song{Source: "spotify", ID: spotifyQualityProbeSongID})
	if err != nil {
		return nil, err
	}
	return []model.AudioQualityOption{spotifyPreviewQuality}, nil
}

func (s *Spotify) GetDownloadURLWithQuality(song *model.Song, quality string) (string, error) {
	normalized, ok := model.NormalizeAudioQuality(quality)
	if !ok {
		return "", fmt.Errorf("unsupported spotify audio quality: %s", quality)
	}
	if normalized != model.AudioQualityAuto && normalized != model.AudioQualityPreview {
		return "", errors.New("spotify full-quality streams require DRM support; only preview is available")
	}
	if song == nil {
		return "", errors.New("song is nil")
	}
	if song.Source != "" && !strings.EqualFold(song.Source, "spotify") {
		return "", errors.New("source mismatch")
	}
	return s.GetDownloadURL(song)
}
