package qianqian

import "github.com/guohuiyuan/music-lib/model"

func GetTopLists() ([]model.TopList, error) {
	return defaultQianqian.GetTopLists()
}

func GetTopListSongs(topListID string) ([]model.Song, error) {
	return defaultQianqian.GetTopListSongs(topListID)
}

func GetHotSongs(limit int) ([]model.Song, error) {
	return defaultQianqian.GetHotSongs(limit)
}

func (p *Qianqian) GetTopLists() ([]model.TopList, error) {
	return nil, model.ErrTopListsUnsupported
}

func (p *Qianqian) GetTopListSongs(topListID string) ([]model.Song, error) {
	return nil, model.ErrTopListsUnsupported
}

func (p *Qianqian) GetHotSongs(limit int) ([]model.Song, error) {
	return nil, model.ErrHotSongsUnsupported
}
