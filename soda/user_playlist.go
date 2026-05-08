package soda

import "github.com/guohuiyuan/music-lib/model"

func GetUserPlaylists(page, limit int) ([]model.Playlist, error) {
	return defaultSoda.GetUserPlaylists(page, limit)
}

func (p *Soda) GetUserPlaylists(page, limit int) ([]model.Playlist, error) {
	return nil, model.ErrUserPlaylistsUnsupported
}
