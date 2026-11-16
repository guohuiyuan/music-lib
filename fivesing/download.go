package fivesing

import (
	"errors"

	"github.com/guohuiyuan/music-lib/model"
)

func GetDownloadURL(s *model.Song) (string, error) { return defaultFivesing.GetDownloadURL(s) }

// GetDownloadURL 获取下载链接
func (f *Fivesing) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "fivesing" {
		return "", errors.New("source mismatch")
	}
	if s.URL != "" {
		return s.URL, nil
	}

	songID, songType, err := fivesingSongIdentity(s)
	if err != nil {
		return "", err
	}

	return f.fetchAudioLink(songID, songType)
}
