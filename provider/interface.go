package provider

import "github.com/guohuiyuan/music-lib/model"

// MusicProvider 定义了所有音乐源必须实现的方法
type MusicProvider interface {
	// Search 搜索歌曲
	Search(keyword string) ([]model.Song, error)
	
	// Parse 解析分享链接，返回单首歌曲详情（包含下载链接）
	Parse(link string) (*model.Song, error)
	
	// GetDownloadURL 获取下载链接（主要用于搜索结果后续获取，Parse 结果通常已包含 URL）
	GetDownloadURL(s *model.Song) (string, error)
	
	// GetLyrics 获取歌词
	GetLyrics(s *model.Song) (string, error)
}