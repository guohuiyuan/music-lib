package kuwo

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	// TopListAPI 酷我排行榜目录页，榜单菜单内嵌在页面 SSR 数据 window.__NUXT__ 中
	TopListAPI = "http://www.kuwo.cn/rankList"
	// TopListSongsAPI 酷我排行榜歌曲接口（旧版榜单服务，无需登录）
	TopListSongsAPI = "http://kbangserver.kuwo.cn/ksong.s"
	// HotSongTopListID 酷我热歌榜 ID，作为「热门单曲」来源
	HotSongTopListID = "16"
	// DefaultHotSongLimit 热门单曲默认返回条数
	DefaultHotSongLimit = 100
	// MaxHotSongLimit 热门单曲最大返回条数
	MaxHotSongLimit = 200
	// kuwoTopListPageSize 榜单歌曲接口单次请求条数（实测 rn=1000 也能返回，这里取 300）
	kuwoTopListPageSize = 300
	// kuwoTopListNuxtMarker 榜单目录页 SSR 数据标记
	kuwoTopListNuxtMarker = "window.__NUXT__"
	// kuwoTopListPageLink 酷我排行榜官方页面
	kuwoTopListPageLink = "http://www.kuwo.cn/rankList"
)

func GetTopLists() ([]model.TopList, error) {
	return defaultKuwo.GetTopLists()
}

func GetTopListSongs(topListID string) ([]model.Song, error) {
	return defaultKuwo.GetTopListSongs(topListID)
}

func GetHotSongs(limit int) ([]model.Song, error) {
	return defaultKuwo.GetHotSongs(limit)
}

// GetTopLists 获取酷我全部排行榜（榜单目录，无需登录）。
// 酷我 Web 的榜单菜单接口需要 CSRF 校验，这里解析榜单目录页 SSR 数据里的 bangMenu。
func (k *Kuwo) GetTopLists() ([]model.TopList, error) {
	body, err := utils.Get(TopListAPI, kuwoTopListRequestOptions(k.cookie)...)
	if err != nil {
		return nil, err
	}

	return parseKuwoTopLists(body)
}

// GetTopListSongs 获取指定排行榜的全部歌曲，按榜单名次排序，名次写入 Song.Extra["rank"]。
func (k *Kuwo) GetTopListSongs(topListID string) ([]model.Song, error) {
	return k.fetchTopListSongs(topListID, 0)
}

// GetHotSongs 获取酷我热歌榜（热门单曲），limit <= 0 时返回默认条数。
func (k *Kuwo) GetHotSongs(limit int) ([]model.Song, error) {
	return k.fetchTopListSongs(HotSongTopListID, normalizeKuwoHotSongLimit(limit))
}

func kuwoTopListRequestOptions(cookie string) []utils.RequestOption {
	opts := []utils.RequestOption{
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", kuwoTopListPageLink),
		utils.WithRandomIPHeader(),
	}
	if strings.TrimSpace(cookie) != "" {
		opts = append(opts, utils.WithHeader("Cookie", cookie))
	}
	return opts
}

func (k *Kuwo) fetchTopListSongs(topListID string, limit int) ([]model.Song, error) {
	topListID = strings.TrimSpace(topListID)
	if topListID == "" {
		return nil, errors.New("kuwo toplist id is empty")
	}
	if !isKuwoTopListID(topListID) {
		return nil, fmt.Errorf("invalid kuwo toplist id: %s", topListID)
	}

	pageSize := kuwoTopListPageSize
	if limit > 0 && limit < pageSize {
		pageSize = limit
	}

	songs := make([]model.Song, 0, pageSize)
	for page := 0; ; page++ {
		pageSongs, total, err := k.fetchTopListSongsPage(topListID, page, pageSize)
		if err != nil {
			return nil, err
		}
		if len(pageSongs) == 0 {
			break
		}

		songs = append(songs, pageSongs...)
		if limit > 0 && len(songs) >= limit {
			songs = songs[:limit]
			break
		}
		if total > 0 && len(songs) >= total {
			break
		}
		if len(pageSongs) < pageSize {
			break
		}
	}

	if len(songs) == 0 {
		return nil, fmt.Errorf("kuwo toplist %s has no songs", topListID)
	}

	return songs, nil
}

// fetchTopListSongsPage 拉取榜单第 page 页歌曲（旧接口偏移量为 pn*rn），同时返回曲目总数。
func (k *Kuwo) fetchTopListSongsPage(topListID string, page, pageSize int) ([]model.Song, int, error) {
	params := url.Values{}
	params.Set("from", "pc")
	params.Set("fmt", "json")
	params.Set("pn", strconv.Itoa(page))
	params.Set("rn", strconv.Itoa(pageSize))
	params.Set("type", "bang")
	params.Set("data", "content")
	params.Set("id", topListID)
	params.Set("show_copyright_off", "1")
	params.Set("pcmp4", "1")

	body, err := utils.Get(TopListSongsAPI+"?"+params.Encode(), kuwoTopListRequestOptions(k.cookie)...)
	if err != nil {
		return nil, 0, err
	}

	return parseKuwoTopListSongs(body, topListID, page*pageSize)
}

type kuwoBangSong struct {
	ID           interface{} `json:"id"`
	Name         string      `json:"name"`
	Artist       string      `json:"artist"`
	Album        string      `json:"album"`
	AlbumID      interface{} `json:"albumid"`
	ArtistID     interface{} `json:"artistid"`
	Duration     interface{} `json:"duration"`
	SongDuration interface{} `json:"song_duration"`
	Formats      string      `json:"formats"`
	Score100     interface{} `json:"score100"`
	Trend        string      `json:"trend"`
	RankChange   interface{} `json:"rank_change"`
	HighestRank  interface{} `json:"highest_rank"`
	IsNew        interface{} `json:"isnew"`
	IsRedSong    interface{} `json:"isRedSong"`
	Pay          interface{} `json:"pay"`
	Copyright    interface{} `json:"copyright"`
	Online       interface{} `json:"online"`
}

// parseKuwoTopListSongs 解析榜单歌曲接口响应，offset 为该页第一首歌在榜单中的前置数量。
func parseKuwoTopListSongs(body []byte, topListID string, offset int) ([]model.Song, int, error) {
	var resp struct {
		Name      string         `json:"name"`
		Num       interface{}    `json:"num"`
		MusicList []kuwoBangSong `json:"musiclist"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, fmt.Errorf("kuwo toplist json parse error: %w", err)
	}

	total := parseKuwoAnyInt(resp.Num)
	songs := make([]model.Song, 0, len(resp.MusicList))
	for i, item := range resp.MusicList {
		name := normalizeKuwoText(item.Name)
		rid := parseKuwoAnyString(item.ID)
		if name == "" || rid == "" {
			continue
		}

		duration := parseKuwoAnyInt(item.SongDuration)
		if duration <= 0 {
			duration = parseKuwoAnyInt(item.Duration)
		}

		song := model.Song{
			Source:   "kuwo",
			ID:       rid,
			Name:     name,
			Artist:   normalizeKuwoText(item.Artist),
			Album:    normalizeKuwoText(item.Album),
			AlbumID:  parseKuwoAnyString(item.AlbumID),
			Duration: duration,
			Link:     fmt.Sprintf("http://www.kuwo.cn/play_detail/%s", rid),
			Extra: map[string]string{
				"rid":        rid,
				"toplist_id": topListID,
				"rank":       strconv.Itoa(offset + i + 1),
			},
		}

		extra := map[string]string{
			"artist_id":    parseKuwoAnyString(item.ArtistID),
			"formats":      strings.TrimSpace(item.Formats),
			"trend":        strings.TrimSpace(item.Trend),
			"rank_change":  parseKuwoAnyString(item.RankChange),
			"highest_rank": parseKuwoAnyString(item.HighestRank),
			"score":        parseKuwoAnyString(item.Score100),
			"is_new":       parseKuwoAnyString(item.IsNew),
			"is_red_song":  parseKuwoAnyString(item.IsRedSong),
			"pay":          parseKuwoAnyString(item.Pay),
			"copyright":    parseKuwoAnyString(item.Copyright),
			"online":       parseKuwoAnyString(item.Online),
		}
		for key, value := range extra {
			if value != "" {
				song.Extra[key] = value
			}
		}

		songs = append(songs, song)
	}

	return songs, total, nil
}

func normalizeKuwoHotSongLimit(limit int) int {
	if limit <= 0 {
		return DefaultHotSongLimit
	}
	if limit > MaxHotSongLimit {
		return MaxHotSongLimit
	}
	return limit
}

func isKuwoTopListID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ==========================================
// 榜单目录（SSR 数据解析）
// ==========================================

type kuwoBangMenuItem struct {
	ID          string
	Name        string
	Cover       string
	Description string
	Frequency   string
	Pub         string
}

// parseKuwoTopLists 解析榜单目录页 window.__NUXT__ 中的 bangMenu。
// 页面里的 sourceid 可能被压缩成 IIFE 形参（如 sourceid:f），因此先还原实参再取值。
func parseKuwoTopLists(body []byte) ([]model.TopList, error) {
	page := string(body)
	marker := strings.Index(page, kuwoTopListNuxtMarker)
	if marker < 0 {
		return nil, errors.New("kuwo toplist menu not found")
	}

	inline := page[marker:]
	if end := strings.Index(inline, "</script>"); end >= 0 {
		inline = inline[:end]
	}

	args, err := parseKuwoNuxtArgs(inline)
	if err != nil {
		return nil, err
	}

	menuPos := strings.Index(inline, "bangMenu:[")
	if menuPos < 0 {
		return nil, errors.New("kuwo toplist menu not found")
	}

	topLists, err := parseKuwoBangMenu(inline[menuPos+len("bangMenu:"):], args)
	if err != nil {
		return nil, err
	}
	if len(topLists) == 0 {
		return nil, errors.New("kuwo toplist menu is empty")
	}

	return topLists, nil
}

// parseKuwoNuxtArgs 解析 IIFE 的形参名与实参值，返回「形参名 -> 实参值」映射。
func parseKuwoNuxtArgs(inline string) (map[string]string, error) {
	header := strings.Index(inline, "(function(")
	if header < 0 {
		return nil, errors.New("kuwo toplist page data is malformed")
	}

	namesStart := header + len("(function(")
	namesEnd := strings.Index(inline[namesStart:], ")")
	if namesEnd < 0 {
		return nil, errors.New("kuwo toplist page data is malformed")
	}
	names := strings.Split(inline[namesStart:namesStart+namesEnd], ",")

	argsStart := strings.LastIndex(inline, "}(")
	if argsStart < 0 {
		return nil, errors.New("kuwo toplist page data is malformed")
	}

	values := make(map[string]string, len(names))
	pos := skipKuwoJSSpace(inline, argsStart+2)
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if pos >= len(inline) || inline[pos] == ')' {
			break
		}
		value, next, err := scanKuwoJSValue(inline, pos, nil)
		if err != nil {
			return nil, err
		}
		values[name] = value
		pos = skipKuwoJSSpace(inline, next)
		if pos < len(inline) && inline[pos] == ',' {
			pos = skipKuwoJSSpace(inline, pos+1)
		}
	}
	if len(values) == 0 {
		return nil, errors.New("kuwo toplist page data is malformed")
	}

	return values, nil
}

// parseKuwoBangMenu 解析 bangMenu 数组，每个元素是一个分组（官方/特色/场景/语言/全球）。
func parseKuwoBangMenu(src string, args map[string]string) ([]model.TopList, error) {
	pos := skipKuwoJSSpace(src, 0)
	if pos >= len(src) || src[pos] != '[' {
		return nil, errors.New("kuwo toplist menu is malformed")
	}
	pos++

	var topLists []model.TopList
	for pos < len(src) {
		pos = skipKuwoJSSpace(src, pos)
		if pos >= len(src) || src[pos] == ']' {
			break
		}
		if src[pos] == ',' {
			pos++
			continue
		}
		if src[pos] != '{' {
			return nil, fmt.Errorf("kuwo toplist menu is malformed near %q", kuwoJSSnippet(src, pos))
		}

		groupName := ""
		itemsRaw := ""
		next, err := parseKuwoJSObject(src, pos, args, func(key, value, _ string) {
			switch key {
			case "name":
				groupName = value
			case "list":
				itemsRaw = value
			}
		})
		if err != nil {
			return nil, err
		}
		pos = next

		if strings.TrimSpace(itemsRaw) == "" {
			continue
		}
		group := strings.TrimSpace(groupName)
		if group == "" {
			group = "其他"
		}

		items, err := parseKuwoBangItems(itemsRaw, args)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			topList := model.TopList{
				ID:              item.ID,
				Name:            item.Name,
				Cover:           normalizeKuwoImageURL(item.Cover),
				Description:     normalizeKuwoText(item.Description),
				UpdateFrequency: firstNonEmpty(item.Frequency, item.Pub),
				Source:          "kuwo",
				Link:            kuwoTopListPageLink,
				Extra: map[string]string{
					"group": group,
				},
			}
			if item.Pub != "" {
				topList.Extra["pub"] = item.Pub
			}
			topLists = append(topLists, topList)
		}
	}

	return topLists, nil
}

// parseKuwoBangItems 解析单个分组下的榜单列表。
func parseKuwoBangItems(src string, args map[string]string) ([]kuwoBangMenuItem, error) {
	pos := skipKuwoJSSpace(src, 0)
	if pos >= len(src) || src[pos] != '[' {
		return nil, errors.New("kuwo toplist group items are malformed")
	}
	pos++

	var items []kuwoBangMenuItem
	for pos < len(src) {
		pos = skipKuwoJSSpace(src, pos)
		if pos >= len(src) || src[pos] == ']' {
			break
		}
		if src[pos] == ',' {
			pos++
			continue
		}
		if src[pos] != '{' {
			return nil, fmt.Errorf("kuwo toplist group items are malformed near %q", kuwoJSSnippet(src, pos))
		}

		item := kuwoBangMenuItem{}
		next, err := parseKuwoJSObject(src, pos, args, func(key, value, _ string) {
			switch key {
			case "sourceid":
				item.ID = value
			case "name":
				item.Name = value
			case "pic":
				item.Cover = value
			case "intro":
				item.Description = value
			case "pub":
				item.Frequency = value
				item.Pub = value
			}
		})
		if err != nil {
			return nil, err
		}
		pos = next

		if item.ID != "" && item.Name != "" {
			items = append(items, item)
		}
	}

	return items, nil
}

// parseKuwoJSObject 解析一个 JS 对象字面量，标量直接取值，数组/对象以原文交给调用方继续解析。
func parseKuwoJSObject(src string, pos int, args map[string]string, visit func(key, value, raw string)) (int, error) {
	if pos >= len(src) || src[pos] != '{' {
		return pos, errors.New("kuwo js object is malformed")
	}
	pos++

	for pos < len(src) {
		pos = skipKuwoJSSpace(src, pos)
		if pos >= len(src) {
			break
		}
		if src[pos] == '}' {
			return pos + 1, nil
		}
		if src[pos] == ',' {
			pos++
			continue
		}

		key, next, err := scanKuwoJSKey(src, pos)
		if err != nil {
			return pos, err
		}
		pos = skipKuwoJSSpace(src, next)
		if pos >= len(src) || src[pos] != ':' {
			return pos, errors.New("kuwo js object key is malformed")
		}
		pos = skipKuwoJSSpace(src, pos+1)

		value, next, err := scanKuwoJSValue(src, pos, args)
		if err != nil {
			return pos, err
		}
		pos = next
		if visit != nil {
			visit(key, value, value)
		}
	}

	return pos, errors.New("kuwo js object is unterminated")
}

func scanKuwoJSKey(src string, pos int) (string, int, error) {
	if pos >= len(src) {
		return "", pos, errors.New("kuwo js key is malformed")
	}
	if src[pos] == '"' || src[pos] == '\'' {
		return scanKuwoJSString(src, pos)
	}

	end := pos
	for end < len(src) && isKuwoJSIdentifierByte(src[end]) {
		end++
	}
	if end == pos {
		return "", pos, errors.New("kuwo js key is malformed")
	}
	return src[pos:end], end, nil
}

// scanKuwoJSValue 读取一个 JS 字面量，identifier 会用 args 还原成实际值。
func scanKuwoJSValue(src string, pos int, args map[string]string) (string, int, error) {
	pos = skipKuwoJSSpace(src, pos)
	if pos >= len(src) {
		return "", pos, errors.New("kuwo js value is malformed")
	}

	switch src[pos] {
	case '"', '\'':
		return scanKuwoJSString(src, pos)
	case '[', '{':
		next, err := skipKuwoJSBlock(src, pos)
		if err != nil {
			return "", pos, err
		}
		return strings.TrimSpace(src[pos:next]), next, nil
	}

	end := pos
	for end < len(src) && isKuwoJSIdentifierByte(src[end]) {
		end++
	}
	if end == pos {
		return "", pos, errors.New("kuwo js value is malformed")
	}
	token := src[pos:end]
	if args != nil {
		if value, ok := args[token]; ok {
			return value, end, nil
		}
	}
	return token, end, nil
}

// scanKuwoJSString 读取 JS 字符串字面量并反转义，返回值和下一个位置。
func scanKuwoJSString(src string, pos int) (string, int, error) {
	quote := src[pos]
	var raw strings.Builder
	for i := pos + 1; i < len(src); i++ {
		switch src[i] {
		case '\\':
			if i+1 >= len(src) {
				return "", i, errors.New("kuwo js string is unterminated")
			}
			raw.WriteByte(src[i])
			raw.WriteByte(src[i+1])
			i++
		case quote:
			return unescapeKuwoJSString(raw.String()), i + 1, nil
		default:
			raw.WriteByte(src[i])
		}
	}
	return "", len(src), errors.New("kuwo js string is unterminated")
}

// skipKuwoJSBlock 跳过一段数组或对象字面量，返回结束后的位置。
func skipKuwoJSBlock(src string, pos int) (int, error) {
	depth := 0
	for i := pos; i < len(src); i++ {
		switch src[i] {
		case '"', '\'':
			_, next, err := scanKuwoJSString(src, i)
			if err != nil {
				return i, err
			}
			i = next - 1
		case '[', '{':
			depth++
		case ']', '}':
			depth--
			if depth <= 0 {
				return i + 1, nil
			}
		}
	}
	return len(src), errors.New("kuwo js block is unterminated")
}

func unescapeKuwoJSString(raw string) string {
	quoted := `"` + strings.ReplaceAll(raw, `"`, `\"`) + `"`
	var decoded string
	if err := json.Unmarshal([]byte(quoted), &decoded); err == nil {
		return decoded
	}

	var b strings.Builder
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' || i+1 >= len(raw) {
			b.WriteByte(raw[i])
			continue
		}
		i++
		switch raw[i] {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'u':
			if i+4 < len(raw) {
				if v, err := strconv.ParseUint(raw[i+1:i+5], 16, 32); err == nil {
					b.WriteRune(rune(v))
					i += 4
					continue
				}
			}
			b.WriteByte('u')
		default:
			b.WriteByte(raw[i])
		}
	}
	return b.String()
}

func skipKuwoJSSpace(src string, pos int) int {
	for pos < len(src) {
		switch src[pos] {
		case ' ', '\t', '\r', '\n':
			pos++
		default:
			return pos
		}
	}
	return pos
}

func isKuwoJSIdentifierByte(ch byte) bool {
	switch {
	case ch >= 'a' && ch <= 'z':
		return true
	case ch >= 'A' && ch <= 'Z':
		return true
	case ch >= '0' && ch <= '9':
		return true
	case ch == '_' || ch == '$' || ch == '.':
		return true
	}
	return false
}

func kuwoJSSnippet(src string, pos int) string {
	if pos < 0 {
		pos = 0
	}
	end := pos + 32
	if end > len(src) {
		end = len(src)
	}
	if pos > len(src) {
		return ""
	}
	return src[pos:end]
}
