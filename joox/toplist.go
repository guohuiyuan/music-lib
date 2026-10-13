package joox

import "github.com/guohuiyuan/music-lib/model"

func GetTopLists() ([]model.TopList, error) {
	return defaultJoox.GetTopLists()
}

func GetTopListSongs(topListID string) ([]model.Song, error) {
	return defaultJoox.GetTopListSongs(topListID)
}

func GetHotSongs(limit int) ([]model.Song, error) {
	return defaultJoox.GetHotSongs(limit)
}

func (p *Joox) GetTopLists() ([]model.TopList, error) {
	return nil, model.ErrTopListsUnsupported
}

func (p *Joox) GetTopListSongs(topListID string) ([]model.Song, error) {
	return nil, model.ErrTopListsUnsupported
}

func (p *Joox) GetHotSongs(limit int) ([]model.Song, error) {
	return nil, model.ErrHotSongsUnsupported
}
