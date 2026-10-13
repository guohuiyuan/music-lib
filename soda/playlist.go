package soda

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/guohuiyuan/music-lib/model"
)

func SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return defaultSoda.SearchPlaylist(keyword)
}

func GetPlaylistSongs(id string) ([]model.Song, error) {
	// 复用 fetchPlaylistDetail，只返回歌曲列表
	_, songs, err := defaultSoda.fetchPlaylistDetail(id)
	return songs, err
}

func ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	return defaultSoda.ParsePlaylist(link)
}

// GetRecommendedPlaylists [新增] 获取推荐歌单 (空实现)
func GetRecommendedPlaylists() ([]model.Playlist, error) {
	return defaultSoda.GetRecommendedPlaylists()
}

func GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	return defaultSoda.GetPlaylistCategories()
}

func GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	return defaultSoda.GetCategoryPlaylists(categoryID, page, limit)
}

func (s *Soda) GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	categories := make([]model.PlaylistCategory, 0, len(sodaPlaylistCategoryTags))
	for _, tag := range sodaPlaylistCategoryTags {
		categoryID := strconv.Itoa(tag.ID)
		categories = append(categories, model.PlaylistCategory{
			Source: "soda",
			ID:     categoryID,
			Name:   tag.Name,
			Group:  tag.Group,
			Extra: map[string]string{
				"sub_channel_id": categoryID,
			},
		})
	}
	return categories, nil
}

// GetCategoryPlaylists 按分类（sub_channel_id）拉取发现页推荐歌单。
// 发现页接口没有游标，单次返回约 6~12 个随机歌单，这里并发拉取多轮后去重，
// 从而支持 page/limit 形式的本地分页；categoryID 为空时默认取「每日推荐」(0)。
func (s *Soda) GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	channel, err := sodaCategoryChannel(categoryID)
	if err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = sodaCategoryPlaylistDefaultLimit
	}

	offset := (page - 1) * limit
	items, err := s.fetchSodaCategoryPlaylistItems(channel, sodaCategoryPlaylistRounds(offset, limit))
	if err != nil {
		return nil, err
	}
	if offset >= len(items) {
		return []model.Playlist{}, nil
	}

	end := offset + limit
	if end > len(items) {
		end = len(items)
	}

	playlists := make([]model.Playlist, 0, end-offset)
	for _, item := range items[offset:end] {
		if playlist := sodaBuildCategoryPlaylist(item, channel); playlist.ID != "" {
			playlists = append(playlists, playlist)
		}
	}
	return playlists, nil
}

func (s *Soda) SearchPlaylist(keyword string) ([]model.Playlist, error) {
	body, err := s.fetchAndroidSearch("playlist", keyword, 1, sodaAndroidSearchPageSize)
	if err != nil {
		return nil, err
	}

	var resp struct {
		ResultGroups []struct {
			Data []struct {
				Entity struct {
					Playlist sodaUserPlaylistItem `json:"playlist"`
				} `json:"entity"`
			} `json:"data"`
		} `json:"result_groups"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda playlist json parse error: %w", err)
	}

	var playlists []model.Playlist
	for _, group := range resp.ResultGroups {
		for _, item := range group.Data {
			pl := item.Entity.Playlist
			if pl.ID == "" {
				continue
			}

			creator := pl.Owner.PublicName
			if creator == "" {
				creator = pl.Owner.Nickname
			}

			playlists = append(playlists, model.Playlist{
				Source:      "soda",
				ID:          pl.ID,
				Name:        pl.Title,
				Cover:       sodaBuildImageURL(pl.URLCover, "~c5_300x300.jpg"),
				TrackCount:  pl.CountTracks,
				Creator:     creator,
				Description: pl.Desc,
				Link:        fmt.Sprintf("https://www.qishui.com/playlist/%s", pl.ID),
			})
		}
	}
	return playlists, nil
}

func (s *Soda) GetPlaylistSongs(id string) ([]model.Song, error) {
	_, songs, err := s.fetchPlaylistDetail(id)
	return songs, err
}

func (s *Soda) ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	playlistID, err := s.extractPlaylistID(link)
	if err != nil || playlistID == "" {
		return nil, nil, errors.New("invalid soda playlist link")
	}
	return s.fetchPlaylistDetail(playlistID)
}

// GetRecommendedPlaylists [新增] 获取推荐歌单 (空实现)
func (s *Soda) GetRecommendedPlaylists() ([]model.Playlist, error) {
	// 汽水音乐目前没有公开的每日推荐歌单 PC 接口
	return nil, errors.New("soda daily recommendation not supported")
}

const (
	// sodaCategoryPlaylistDefaultLimit 分类歌单默认返回条数。
	sodaCategoryPlaylistDefaultLimit = 30
	// sodaCategoryPlaylistItemsPerRound 发现页单轮请求大致返回的歌单数量，用于估算抓取轮数。
	sodaCategoryPlaylistItemsPerRound = 8
	// sodaCategoryPlaylistMinRounds 最少抓取轮数，保证分类列表不会过于单薄。
	sodaCategoryPlaylistMinRounds = 2
	// sodaCategoryPlaylistMaxRounds 最多抓取轮数，避免一次分页请求打太多接口。
	sodaCategoryPlaylistMaxRounds = 5
)

// sodaPlaylistCategoryTag 汽水音乐发现页的歌单分类标签，ID 即接口的 sub_channel_id。
// 该接口没有公开的分类目录，标签与汽水音乐 App 发现页保持一致。
type sodaPlaylistCategoryTag struct {
	ID    int
	Name  string
	Group string
}

var sodaPlaylistCategoryTags = []sodaPlaylistCategoryTag{
	{ID: 0, Name: "每日推荐", Group: "推荐"},
	{ID: 14, Name: "流行", Group: "曲风"},
	{ID: 8, Name: "华语", Group: "曲风"},
	{ID: 9, Name: "欧美", Group: "曲风"},
	{ID: 20, Name: "国风", Group: "曲风"},
	{ID: 18, Name: "民谣", Group: "曲风"},
	{ID: 15, Name: "摇滚", Group: "曲风"},
	{ID: 38, Name: "说唱", Group: "曲风"},
	{ID: 16, Name: "电子", Group: "曲风"},
	{ID: 19, Name: "R&B", Group: "曲风"},
	{ID: 69, Name: "治愈", Group: "场景"},
	{ID: 45, Name: "睡前", Group: "场景"},
	{ID: 40, Name: "学习", Group: "场景"},
}

// sodaCategoryChannel 解析分类 ID，空值回退到「每日推荐」(sub_channel_id=0)。
func sodaCategoryChannel(categoryID string) (int, error) {
	categoryID = strings.TrimSpace(categoryID)
	if categoryID == "" {
		return 0, nil
	}

	channel, err := strconv.Atoi(categoryID)
	if err != nil || channel < 0 {
		return 0, fmt.Errorf("invalid soda playlist category id: %s", categoryID)
	}
	return channel, nil
}

// sodaCategoryPlaylistRounds 根据需要的条数估算抓取轮数。
func sodaCategoryPlaylistRounds(offset, limit int) int {
	needed := offset + limit
	rounds := (needed + sodaCategoryPlaylistItemsPerRound - 1) / sodaCategoryPlaylistItemsPerRound
	if rounds < sodaCategoryPlaylistMinRounds {
		rounds = sodaCategoryPlaylistMinRounds
	}
	if rounds > sodaCategoryPlaylistMaxRounds {
		rounds = sodaCategoryPlaylistMaxRounds
	}
	return rounds
}

// fetchSodaCategoryPlaylistItems 并发抓取多轮发现页歌单并按轮次顺序去重合并。
func (s *Soda) fetchSodaCategoryPlaylistItems(channel, rounds int) ([]sodaUserPlaylistItem, error) {
	if rounds < 1 {
		rounds = 1
	}

	batches := make([][]sodaUserPlaylistItem, rounds)
	errs := make([]error, rounds)

	var wg sync.WaitGroup
	for i := 0; i < rounds; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			items, err := s.fetchSodaRecommendChannel(channel)
			if err != nil {
				errs[index] = err
				return
			}
			batches[index] = items
		}(i)
	}
	wg.Wait()

	seen := make(map[string]bool)
	items := make([]sodaUserPlaylistItem, 0, rounds*sodaCategoryPlaylistItemsPerRound)
	for _, batch := range batches {
		for _, item := range batch {
			if item.ID == "" || seen[item.ID] {
				continue
			}
			seen[item.ID] = true
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		for _, err := range errs {
			if err != nil {
				return nil, err
			}
		}
		return nil, fmt.Errorf("soda category %d has no playlists", channel)
	}
	return items, nil
}

// sodaBuildCategoryPlaylist 把发现页歌单转换成 model.Playlist，并记录来源分类。
func sodaBuildCategoryPlaylist(item sodaUserPlaylistItem, channel int) model.Playlist {
	playlist := sodaBuildPlaylistFromUserItem(item, "", "")
	if playlist.ID == "" {
		return playlist
	}
	if playlist.Extra == nil {
		playlist.Extra = make(map[string]string, 2)
	}
	// 与排行榜里的推荐歌单保持一致：优先展示昵称，其次才是公开名称。
	playlist.Creator = sodaFirstNonEmpty(item.Owner.Nickname, item.Owner.PublicName, playlist.Creator)
	playlist.Extra["sub_channel_id"] = strconv.Itoa(channel)
	playlist.Description = cleanSodaTopListText(playlist.Description, 200)
	return playlist
}
