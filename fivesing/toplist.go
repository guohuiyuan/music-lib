package fivesing

import "github.com/guohuiyuan/music-lib/model"

func GetTopLists() ([]model.TopList, error) {
	return defaultFivesing.GetTopLists()
}

func GetTopListSongs(topListID string) ([]model.Song, error) {
	return defaultFivesing.GetTopListSongs(topListID)
}

func GetHotSongs(limit int) ([]model.Song, error) {
	return defaultFivesing.GetHotSongs(limit)
}

func (p *Fivesing) GetTopLists() ([]model.TopList, error) {
	return nil, model.ErrTopListsUnsupported
}

func (p *Fivesing) GetTopListSongs(topListID string) ([]model.Song, error) {
	return nil, model.ErrTopListsUnsupported
}

func (p *Fivesing) GetHotSongs(limit int) ([]model.Song, error) {
	return nil, model.ErrHotSongsUnsupported
}
