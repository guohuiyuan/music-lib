package qq

import "github.com/guohuiyuan/music-lib/model"

func GetDownloadURL(s *model.Song) (string, error) { return defaultQQ.GetDownloadURL(s) }

// GetDownloadURL 返回下载地址。
func (q *QQ) GetDownloadURL(s *model.Song) (string, error) {
	return q.GetDownloadURLWithQuality(s, string(model.AudioQualityAuto))
}
