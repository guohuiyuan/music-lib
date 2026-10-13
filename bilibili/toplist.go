package bilibili

import "github.com/guohuiyuan/music-lib/model"

func GetTopLists() ([]model.TopList, error) {
	return defaultBilibili.GetTopLists()
}

func GetTopListSongs(topListID string) ([]model.Song, error) {
	return defaultBilibili.GetTopListSongs(topListID)
}

func GetHotSongs(limit int) ([]model.Song, error) {
	return defaultBilibili.GetHotSongs(limit)
}

func (b *Bilibili) GetTopLists() ([]model.TopList, error) {
	return nil, model.ErrTopListsUnsupported
}

func (b *Bilibili) GetTopListSongs(topListID string) ([]model.Song, error) {
	return nil, model.ErrTopListsUnsupported
}

func (b *Bilibili) GetHotSongs(limit int) ([]model.Song, error) {
	return nil, model.ErrHotSongsUnsupported
}
