package model

import (
	"fmt"

	"github.com/guohuiyuan/music-lib/utils"
)

// Song 是所有音乐源通用的歌曲结构
type Song struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	AlbumID  string `json:"album_id"` // 某些源特有，用于获取封面
	Duration int    `json:"duration"` // 秒
	Size     int64  `json:"size"`     // 文件大小 (字节)
	Bitrate  int    `json:"bitrate"`  // 码率 (kbps)
	Source   string `json:"source"`   // kugou, netease, qq, bilibili...
	URL      string `json:"url"`      // 真实音频文件下载链接
	Ext      string `json:"ext"`      // 文件后缀 (mp3, flac...)
	Cover    string `json:"cover"`    // 封面图片链接

	// [新增] 歌曲原始链接 (例如网页地址)
	Link string `json:"link"`

	// 用于存储源特有的元数据，避免解析 ID
	Extra map[string]string `json:"extra,omitempty"`
}

// Playlist 是所有音乐源通用的歌单结构 [新增]
type Playlist struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Cover       string `json:"cover"`
	TrackCount  int    `json:"track_count"`
	PlayCount   int    `json:"play_count"`
	Creator     string `json:"creator"`
	Description string `json:"description"`
	Source      string `json:"source"`
}

// FormatDuration 格式化时长 (e.g. 03:45)
func (s *Song) FormatDuration() string {
	if s.Duration == 0 {
		return "-"
	}
	min := s.Duration / 60
	sec := s.Duration % 60
	return fmt.Sprintf("%02d:%02d", min, sec)
}

// FormatSize 格式化大小 (e.g. 4.5 MB)
func (s *Song) FormatSize() string {
	if s.Size == 0 {
		return "-"
	}
	mb := float64(s.Size) / 1024 / 1024
	return fmt.Sprintf("%.2f MB", mb)
}

// FormatBitrate 格式化码率 (e.g. 320 kbps)
func (s *Song) FormatBitrate() string {
	if s.Bitrate == 0 {
		return "-"
	}
	return fmt.Sprintf("%d kbps", s.Bitrate)
}

// Filename 生成清晰的文件名 (歌手 - 歌名.ext)
func (s *Song) Filename() string {
	ext := s.Ext
	if ext == "" {
		ext = "mp3" // 默认
	}
	return utils.SanitizeFilename(fmt.Sprintf("%s - %s.%s", s.Artist, s.Name, ext))
}

// Display 用于简单的日志打印
func (s *Song) Display() string {
	return s.Name + " - " + s.Artist
}