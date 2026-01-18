package model

// Song 是所有音乐源通用的歌曲结构
type Song struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	AlbumID  string `json:"album_id"` // 某些源特有
	Duration int    `json:"duration"` // 秒
	Source   string `json:"source"`   // kugou, netease, qq
	URL      string `json:"url"`      // 真实下载链接
	Size     int64  `json:"size"`     // 文件大小
}

// Display 用于简单的日志打印
func (s *Song) Display() string {
	return s.Name + " - " + s.Artist
}
