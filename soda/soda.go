package soda

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
)

type Soda struct {
	cookie string
}

func New(cookie string) *Soda { return &Soda{cookie: cookie} }
var defaultSoda = New("")
func Search(keyword string) ([]model.Song, error) { return defaultSoda.Search(keyword) }
func GetDownloadInfo(s *model.Song) (*DownloadInfo, error) { return defaultSoda.GetDownloadInfo(s) }
func GetDownloadURL(s *model.Song) (string, error) { return defaultSoda.GetDownloadURL(s) }
func Download(s *model.Song, outputPath string) error { return defaultSoda.Download(s, outputPath) }
func GetLyrics(s *model.Song) (string, error) { return defaultSoda.GetLyrics(s) }
func Parse(link string) (*model.Song, error) { return defaultSoda.Parse(link) }

// Search 搜索歌曲
func (s *Soda) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("q", keyword)
	params.Set("cursor", "0")
	params.Set("search_method", "input")
	params.Set("aid", "386088")
	params.Set("device_platform", "web")
	params.Set("channel", "pc_web")

	apiURL := "https://api.qishui.com/luna/pc/search/track?" + params.Encode()
	body, err := utils.Get(apiURL, 
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", s.cookie),
	)
	if err != nil { return nil, err }

	var resp struct {
		ResultGroups []struct {
			Data []struct {
				Entity struct {
					Track struct {
						ID       string `json:"id"`
						Name     string `json:"name"`
						Duration int    `json:"duration"`
						Artists  []struct { Name string `json:"name"` } `json:"artists"`
						Album struct {
							Name     string `json:"name"`
							UrlCover struct { Urls []string `json:"urls"`; Uri  string   `json:"uri"` } `json:"url_cover"`
						} `json:"album"`
						QualityMap map[string]struct { DownloadDetail *struct { NeedVip bool `json:"need_vip"` } `json:"download_detail"` } `json:"quality_map"`
						BitRates []struct { Size int64 `json:"size"`; Quality string `json:"quality"` } `json:"bit_rates"`
					} `json:"track"`
				} `json:"entity"`
			} `json:"data"`
		} `json:"result_groups"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda search json parse error: %w", err)
	}
	if len(resp.ResultGroups) == 0 { return nil, nil }

	var songs []model.Song
	for _, item := range resp.ResultGroups[0].Data {
		track := item.Entity.Track
		if track.ID == "" { continue }

		var maxFreeSize int64 
		var maxVipSize int64  

		for _, br := range track.BitRates {
			isVip := false
			if qInfo, ok := track.QualityMap[br.Quality]; ok && qInfo.DownloadDetail != nil {
				if qInfo.DownloadDetail.NeedVip { isVip = true }
			}
			if !isVip {
				if br.Size > maxFreeSize { maxFreeSize = br.Size }
			} else {
				if br.Size > maxVipSize { maxVipSize = br.Size }
			}
		}

		var displaySize int64
		if maxFreeSize > 0 { displaySize = maxFreeSize } else { displaySize = maxVipSize }
		if displaySize == 0 { continue }

		var artistNames []string
		for _, ar := range track.Artists { artistNames = append(artistNames, ar.Name) }

		var cover string
		if len(track.Album.UrlCover.Urls) > 0 {
			domain := track.Album.UrlCover.Urls[0]
			uri := track.Album.UrlCover.Uri
			if domain != "" && uri != "" { cover = domain + uri + "~c5_375x375.jpg" }
		}

		bitrate := 0
		seconds := track.Duration / 1000
		if seconds > 0 && displaySize > 0 { bitrate = int(displaySize * 8 / 1000 / int64(seconds)) }

		songs = append(songs, model.Song{
			Source:   "soda",
			ID:       track.ID,
			Name:     track.Name,
			Artist:   strings.Join(artistNames, "、"),
			Album:    track.Album.Name,
			Duration: track.Duration / 1000,
			Size:     displaySize,
			Bitrate:  bitrate,
			Cover:    cover,
			Link:     fmt.Sprintf("https://www.qishui.com/track/%s", track.ID),
			Extra: map[string]string{
				"track_id": track.ID,
			},
		})
	}
	return songs, nil
}

// Parse 解析链接并获取完整信息
func (s *Soda) Parse(link string) (*model.Song, error) {
	// 1. 提取 TrackID
	// 支持格式: https://www.qishui.com/track/7132890123
	re := regexp.MustCompile(`track/(\d+)`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 2 {
		return nil, errors.New("invalid soda link")
	}
	trackID := matches[1]

	// 2. 调用内部方法获取详情 (Metadata + DownloadInfo)
	return s.fetchSongDetail(trackID)
}

// fetchSongDetail 内部方法：获取详情
func (s *Soda) fetchSongDetail(trackID string) (*model.Song, error) {
	params := url.Values{}
	params.Set("track_id", trackID)
	params.Set("media_type", "track")

	v2URL := "https://api.qishui.com/luna/pc/track_v2?" + params.Encode()
	v2Body, err := utils.Get(v2URL, 
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", s.cookie),
	)
	if err != nil { return nil, err }

	var v2Resp struct {
		TrackInfo struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Duration int    `json:"duration"`
			Artists  []struct { Name string `json:"name"` } `json:"artists"`
			Album    struct {
				Name     string `json:"name"`
				UrlCover struct { Urls []string `json:"urls"`; Uri string `json:"uri"` } `json:"url_cover"`
			} `json:"album"`
		} `json:"track_info"`
		TrackPlayer struct { URLPlayerInfo string `json:"url_player_info"` } `json:"track_player"`
	}
	if err := json.Unmarshal(v2Body, &v2Resp); err != nil { return nil, fmt.Errorf("parse track_v2 response error: %w", err) }
	
	if v2Resp.TrackInfo.ID == "" {
		return nil, errors.New("track info not found")
	}

	// 构造基本 Song
	info := v2Resp.TrackInfo
	var artistNames []string
	for _, ar := range info.Artists { artistNames = append(artistNames, ar.Name) }

	var cover string
	if len(info.Album.UrlCover.Urls) > 0 {
		domain := info.Album.UrlCover.Urls[0]
		uri := info.Album.UrlCover.Uri
		if domain != "" && uri != "" { cover = domain + uri + "~c5_375x375.jpg" }
	}

	song := &model.Song{
		Source:   "soda",
		ID:       info.ID,
		Name:     info.Name,
		Artist:   strings.Join(artistNames, "、"),
		Album:    info.Album.Name,
		Duration: info.Duration / 1000,
		Cover:    cover,
		Link:     fmt.Sprintf("https://www.qishui.com/track/%s", info.ID),
		Extra: map[string]string{
			"track_id": info.ID,
		},
	}

	// 尝试获取 URL
	// 这里不直接调用 GetDownloadInfo，因为 v2Resp 已经包含了 url_player_info，可以减少一次 API 调用
	if v2Resp.TrackPlayer.URLPlayerInfo != "" {
		dInfo, err := s.fetchPlayerInfo(v2Resp.TrackPlayer.URLPlayerInfo)
		if err == nil {
			song.URL = dInfo.URL + "#auth=" + url.QueryEscape(dInfo.PlayAuth)
			song.Size = dInfo.Size
			song.Ext = dInfo.Format
			if song.Duration > 0 && dInfo.Size > 0 {
				song.Bitrate = int(dInfo.Size * 8 / 1000 / int64(song.Duration))
			}
		}
	}

	return song, nil
}

type DownloadInfo struct {
	URL      string
	PlayAuth string
	Format   string
	Size     int64
}

// GetDownloadInfo 获取下载信息
func (s *Soda) GetDownloadInfo(song *model.Song) (*DownloadInfo, error) {
	if song.Source != "soda" { return nil, errors.New("source mismatch") }

	trackID := song.ID
	if song.Extra != nil && song.Extra["track_id"] != "" {
		trackID = song.Extra["track_id"]
	}

	params := url.Values{}
	params.Set("track_id", trackID)
	params.Set("media_type", "track")

	v2URL := "https://api.qishui.com/luna/pc/track_v2?" + params.Encode()
	v2Body, err := utils.Get(v2URL, 
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", s.cookie),
	)
	if err != nil { return nil, err }

	var v2Resp struct {
		TrackPlayer struct { URLPlayerInfo string `json:"url_player_info"` } `json:"track_player"`
	}
	if err := json.Unmarshal(v2Body, &v2Resp); err != nil { return nil, fmt.Errorf("parse track_v2 response error: %w", err) }
	if v2Resp.TrackPlayer.URLPlayerInfo == "" { return nil, errors.New("player info url not found") }

	return s.fetchPlayerInfo(v2Resp.TrackPlayer.URLPlayerInfo)
}

// fetchPlayerInfo 内部辅助：解析 url_player_info
func (s *Soda) fetchPlayerInfo(playerInfoURL string) (*DownloadInfo, error) {
	infoBody, err := utils.Get(playerInfoURL, 
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", s.cookie),
	)
	if err != nil { return nil, err }

	var infoResp struct {
		Result struct {
			Data struct {
				PlayInfoList []struct {
					MainPlayUrl   string `json:"MainPlayUrl"`
					BackupPlayUrl string `json:"BackupPlayUrl"`
					PlayAuth      string `json:"PlayAuth"`
					Size          int64  `json:"Size"`
					Bitrate       int    `json:"Bitrate"`
					Format        string `json:"Format"`
				} `json:"PlayInfoList"`
			} `json:"Data"`
		} `json:"Result"`
	}
	if err := json.Unmarshal(infoBody, &infoResp); err != nil { return nil, fmt.Errorf("parse play info response error: %w", err) }

	list := infoResp.Result.Data.PlayInfoList
	if len(list) == 0 { return nil, errors.New("no audio stream found") }

	sort.Slice(list, func(i, j int) bool {
		if list[i].Size != list[j].Size { return list[i].Size > list[j].Size }
		return list[i].Bitrate > list[j].Bitrate
	})

	best := list[0]
	downloadURL := best.MainPlayUrl
	if downloadURL == "" { downloadURL = best.BackupPlayUrl }
	if downloadURL == "" { return nil, errors.New("invalid download url") }

	return &DownloadInfo{ URL: downloadURL, PlayAuth: best.PlayAuth, Format: best.Format, Size: best.Size }, nil
}

// GetDownloadURL 返回下载链接
func (s *Soda) GetDownloadURL(song *model.Song) (string, error) {
	info, err := s.GetDownloadInfo(song)
	if err != nil { return "", err }
	return info.URL + "#auth=" + url.QueryEscape(info.PlayAuth), nil
}

// Download 下载并解密歌曲
func (s *Soda) Download(song *model.Song, outputPath string) error {
	info, err := s.GetDownloadInfo(song)
	if err != nil { return fmt.Errorf("get download info failed: %w", err) }

	client := &http.Client{}
	req, err := http.NewRequest("GET", info.URL, nil)
	if err != nil { return err }
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return fmt.Errorf("download status: %d", resp.StatusCode) }

	encryptedData, err := io.ReadAll(resp.Body)
	if err != nil { return err }

	decryptedData, err := DecryptAudio(encryptedData, info.PlayAuth)
	if err != nil { return fmt.Errorf("decrypt failed: %w", err) }

	err = os.WriteFile(outputPath, decryptedData, 0644)
	if err != nil { return err }
	return nil
}

// DecryptAudio 解密逻辑
func DecryptAudio(fileData []byte, playAuth string) ([]byte, error) {
	hexKey, err := extractKey(playAuth)
	if err != nil {
		return nil, err
	}
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, err
	}

	moov, err := findBox(fileData, "moov", 0, len(fileData))
	if err != nil {
		return nil, errors.New("moov box not found")
	}

	stbl, err := findBox(fileData, "stbl", moov.offset, moov.offset+moov.size)
	if err != nil {
		trak, _ := findBox(fileData, "trak", moov.offset+8, moov.offset+moov.size)
		if trak != nil {
			mdia, _ := findBox(fileData, "mdia", trak.offset+8, trak.offset+trak.size)
			if mdia != nil {
				minf, _ := findBox(fileData, "minf", mdia.offset+8, mdia.offset+mdia.size)
				if minf != nil {
					stbl, _ = findBox(fileData, "stbl", minf.offset+8, minf.offset+minf.size)
				}
			}
		}
	}
	if stbl == nil {
		return nil, errors.New("stbl box not found")
	}

	stsz, err := findBox(fileData, "stsz", stbl.offset+8, stbl.offset+stbl.size)
	if err != nil {
		return nil, errors.New("stsz box not found")
	}
	sampleSizes := parseStsz(stsz.data)

	senc, err := findBox(fileData, "senc", moov.offset+8, moov.offset+moov.size)
	if err != nil {
		senc, err = findBox(fileData, "senc", stbl.offset+8, stbl.offset+stbl.size)
	}
	if err != nil {
		return nil, errors.New("senc box not found")
	}
	ivs := parseSenc(senc.data)

	mdat, err := findBox(fileData, "mdat", 0, len(fileData))
	if err != nil {
		return nil, errors.New("mdat box not found")
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, err
	}

	decryptedData := make([]byte, len(fileData))
	copy(decryptedData, fileData)

	readPtr := mdat.offset + 8
	decryptedMdat := make([]byte, 0, mdat.size-8)

	for i := 0; i < len(sampleSizes); i++ {
		size := int(sampleSizes[i])
		if readPtr+size > len(decryptedData) {
			break
		}
		chunk := decryptedData[readPtr : readPtr+size]

		if i < len(ivs) {
			iv := ivs[i]
			if len(iv) < 16 {
				padded := make([]byte, 16)
				copy(padded, iv)
				iv = padded
			}
			stream := cipher.NewCTR(block, iv)
			dst := make([]byte, size)
			stream.XORKeyStream(dst, chunk)
			decryptedMdat = append(decryptedMdat, dst...)
		} else {
			decryptedMdat = append(decryptedMdat, chunk...)
		}
		readPtr += size
	}

	if len(decryptedMdat) == int(mdat.size)-8 {
		copy(decryptedData[mdat.offset+8:], decryptedMdat)
	} else {
		return nil, errors.New("decrypted size mismatch")
	}

	stsd, err := findBox(fileData, "stsd", stbl.offset+8, stbl.offset+stbl.size)
	if err == nil {
		stsdOffset := stsd.offset
		stsdData := decryptedData[stsdOffset : stsdOffset+stsd.size]
		if idx := bytes.Index(stsdData, []byte("enca")); idx != -1 {
			copy(stsdData[idx:], []byte("mp4a"))
			copy(decryptedData[stsdOffset:], stsdData)
		}
	}

	return decryptedData, nil
}

type mp4Box struct {
	offset int
	size   int
	data   []byte
}

func findBox(data []byte, boxType string, start, end int) (*mp4Box, error) {
	if end > len(data) {
		end = len(data)
	}
	pos := start
	target := []byte(boxType)
	for pos+8 <= end {
		size := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		if size < 8 {
			break
		}
		if bytes.Equal(data[pos+4:pos+8], target) {
			return &mp4Box{offset: pos, size: size, data: data[pos+8 : pos+size]}, nil
		}
		pos += size
	}
	return nil, errors.New("box not found")
}

func parseStsz(data []byte) []uint32 {
	if len(data) < 12 {
		return nil
	}
	sampleSizeFixed := binary.BigEndian.Uint32(data[4:8])
	sampleCount := int(binary.BigEndian.Uint32(data[8:12]))
	sizes := make([]uint32, sampleCount)
	if sampleSizeFixed != 0 {
		for i := 0; i < sampleCount; i++ {
			sizes[i] = sampleSizeFixed
		}
	} else {
		for i := 0; i < sampleCount; i++ {
			if 12+i*4+4 <= len(data) {
				sizes[i] = binary.BigEndian.Uint32(data[12+i*4 : 12+i*4+4])
			}
		}
	}
	return sizes
}

func parseSenc(data []byte) [][]byte {
	if len(data) < 8 {
		return nil
	}
	flags := binary.BigEndian.Uint32(data[0:4]) & 0x00FFFFFF
	sampleCount := int(binary.BigEndian.Uint32(data[4:8]))
	ivs := make([][]byte, 0, sampleCount)
	ptr := 8
	hasSubsamples := (flags & 0x02) != 0
	for i := 0; i < sampleCount; i++ {
		if ptr+8 > len(data) {
			break
		}
		ivs = append(ivs, data[ptr:ptr+8])
		ptr += 8
		if hasSubsamples {
			if ptr+2 > len(data) {
				break
			}
			subCount := int(binary.BigEndian.Uint16(data[ptr : ptr+2]))
			ptr += 2 + (subCount * 6)
		}
	}
	return ivs
}

func bitcount(n int) int {
	u := uint32(n)
	u = u & 0xFFFFFFFF
	u = u - ((u >> 1) & 0x55555555)
	u = (u & 0x33333333) + ((u >> 2) & 0x33333333)
	return int((((u + (u >> 4)) & 0xF0F0F0F) * 0x1010101) >> 24)
}

func decodeBase36(c byte) int {
	if c >= '0' && c <= '9' {
		return int(c - '0')
	}
	if c >= 'a' && c <= 'z' {
		return int(c - 'a' + 10)
	}
	return 0xFF
}

func decryptSpadeInner(keyBytes []byte) []byte {
	result := make([]byte, len(keyBytes))
	buff := append([]byte{0xFA, 0x55}, keyBytes...)
	for i := 0; i < len(result); i++ {
		v := int(keyBytes[i]^buff[i]) - bitcount(i) - 21
		for v < 0 {
			v += 255
		}
		result[i] = byte(v)
	}
	return result
}

func extractKey(playAuth string) (string, error) {
	binaryStr, err := base64.StdEncoding.DecodeString(playAuth)
	if err != nil {
		return "", err
	}
	bytesData := []byte(binaryStr)
	if len(bytesData) < 3 {
		return "", errors.New("auth data too short")
	}
	paddingLen := int((bytesData[0] ^ bytesData[1] ^ bytesData[2]) - 48)
	if len(bytesData) < paddingLen+2 {
		return "", errors.New("invalid padding length")
	}
	innerInput := bytesData[1 : len(bytesData)-paddingLen]
	tmpBuff := decryptSpadeInner(innerInput)
	if len(tmpBuff) == 0 {
		return "", errors.New("decryption failed")
	}
	skipBytes := decodeBase36(tmpBuff[0])
	endIndex := 1 + (len(bytesData) - paddingLen - 2) - skipBytes
	if endIndex > len(tmpBuff) || endIndex < 1 {
		return "", errors.New("index out of bounds")
	}
	return string(tmpBuff[1:endIndex]), nil
}

// GetLyrics 获取歌词
func (s *Soda) GetLyrics(song *model.Song) (string, error) {
	if song.Source != "soda" { return "", errors.New("source mismatch") }

	trackID := song.ID
	if song.Extra != nil && song.Extra["track_id"] != "" {
		trackID = song.Extra["track_id"]
	}

	params := url.Values{}
	params.Set("track_id", trackID)
	params.Set("media_type", "track")

	v2URL := "https://api.qishui.com/luna/pc/track_v2?" + params.Encode()
	body, err := utils.Get(v2URL, 
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", s.cookie),
	)
	if err != nil { return "", fmt.Errorf("failed to fetch lyric API: %w", err) }

	var resp struct {
		Lyric struct { Content string `json:"content"` } `json:"lyric"`
	}
	if err := json.Unmarshal(body, &resp); err != nil { return "", fmt.Errorf("failed to parse lyric JSON: %w", err) }
	if resp.Lyric.Content == "" { return "", nil }

	return parseSodaLyric(resp.Lyric.Content), nil
}

func parseSodaLyric(raw string) string {
	var sb strings.Builder
	lineRegex := regexp.MustCompile(`^\[(\d+),(\d+)\](.*)$`)
	wordRegex := regexp.MustCompile(`<[^>]+>`)

	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matches := lineRegex.FindStringSubmatch(line)
		if len(matches) >= 4 {
			startTimeStr := matches[1]
			content := matches[3]

			cleanContent := wordRegex.ReplaceAllString(content, "")

			startTime, _ := strconv.Atoi(startTimeStr)
			minutes := startTime / 60000
			seconds := (startTime % 60000) / 1000
			millis := (startTime % 1000) / 10 

			sb.WriteString(fmt.Sprintf("[%02d:%02d.%02d]%s\n", minutes, seconds, millis, cleanContent))
		}
	}
	return sb.String()
}