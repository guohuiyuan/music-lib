package bilibili

import (
	"errors"

	"github.com/guohuiyuan/music-lib/model"
)

func GetDownloadURL(s *model.Song) (string, error) { return defaultBilibili.GetDownloadURL(s) }

// GetDownloadURL 获取下载链接
func (b *Bilibili) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "bilibili" {
		return "", errors.New("source mismatch")
	}

	if s.URL != "" {
		return s.URL, nil
	}

	bvid, cid, err := bilibiliSongIdentity(s)
	if err != nil {
		return "", err
	}

	isVip, _ := b.IsVipAccount()
	return b.fetchAudioURL(bvid, cid, isVip)
}
