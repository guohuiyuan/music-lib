package jamendo

import "github.com/guohuiyuan/music-lib/model"

func GetTopLists() ([]model.TopList, error) {
	return defaultJamendo.GetTopLists()
}

func GetTopListSongs(topListID string) ([]model.Song, error) {
	return defaultJamendo.GetTopListSongs(topListID)
}

func GetHotSongs(limit int) ([]model.Song, error) {
	return defaultJamendo.GetHotSongs(limit)
}

func (p *Jamendo) GetTopLists() ([]model.TopList, error) {
	return nil, model.ErrTopListsUnsupported
}

func (p *Jamendo) GetTopListSongs(topListID string) ([]model.Song, error) {
	return nil, model.ErrTopListsUnsupported
}

func (p *Jamendo) GetHotSongs(limit int) ([]model.Song, error) {
	return nil, model.ErrHotSongsUnsupported
}
