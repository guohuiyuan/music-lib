package provider

import "github.com/guohuiyuan/music-lib/model"

type SongSearcher interface {
	Search(keyword string) ([]model.Song, error)
}

type SongParser interface {
	Parse(link string) (*model.Song, error)
}

type SongDownloader interface {
	GetDownloadURL(s *model.Song) (string, error)
}

type LyricProvider interface {
	GetLyrics(s *model.Song) (string, error)
}

type MusicProvider interface {
	SongSearcher
	SongParser
	SongDownloader
	LyricProvider
}

type AlbumProvider interface {
	SearchAlbum(keyword string) ([]model.Playlist, error)
	GetAlbumSongs(id string) ([]model.Song, error)
	ParseAlbum(link string) (*model.Playlist, []model.Song, error)
}

type PlaylistProvider interface {
	SearchPlaylist(keyword string) ([]model.Playlist, error)
	GetPlaylistSongs(id string) ([]model.Song, error)
	ParsePlaylist(link string) (*model.Playlist, []model.Song, error)
}

type RecommendedPlaylistProvider interface {
	GetRecommendedPlaylists() ([]model.Playlist, error)
}

// TopListProvider 排行榜能力：榜单列表 + 榜单歌曲
type TopListProvider interface {
	GetTopLists() ([]model.TopList, error)
	GetTopListSongs(topListID string) ([]model.Song, error)
}

// HotSongProvider 热门单曲能力
type HotSongProvider interface {
	GetHotSongs(limit int) ([]model.Song, error)
}

// RankingProvider 聚合排行榜与热门单曲能力
type RankingProvider interface {
	TopListProvider
	HotSongProvider
}

type PlaylistCategoryProvider interface {
	GetPlaylistCategories() ([]model.PlaylistCategory, error)
	GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error)
}

type UserPlaylistProvider interface {
	GetUserPlaylists(page, limit int) ([]model.Playlist, error)
}

type QRLoginProvider interface {
	CreateQRLogin() (*model.QRLoginSession, error)
	CheckQRLogin(key string) (*model.QRLoginResult, error)
}

type FullPlaylistProvider interface {
	PlaylistProvider
	RecommendedPlaylistProvider
	PlaylistCategoryProvider
}

type FullMusicProvider interface {
	MusicProvider
	AlbumProvider
	FullPlaylistProvider
}
