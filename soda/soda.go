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
	"net/url"
	"sort"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
)

// Search 搜索歌曲
func Search(keyword string) ([]model.Song, error) {
	// 1. 构造搜索参数
	params := url.Values{}
	params.Set("q", keyword)
	params.Set("cursor", "0")
	params.Set("search_method", "input")
	params.Set("aid", "386088") // 汽水音乐 Web AppID
	params.Set("device_platform", "web")
	params.Set("channel", "pc_web")

	apiURL := "https://api.qishui.com/luna/pc/search/track?" + params.Encode()

	// 2. 发送请求
	body, err := utils.Get(apiURL, utils.WithHeader("User-Agent", UserAgent))
	if err != nil {
		return nil, err
	}

	// 3. 解析 JSON
	var resp struct {
		ResultGroups []struct {
			Data []struct {
				Entity struct {
					Track struct {
						ID       string `json:"id"`
						Name     string `json:"name"`
						Duration int    `json:"duration"`
						Artists  []struct {
							Name string `json:"name"`
						} `json:"artists"`
						Album struct {
							Name     string `json:"name"`
							UrlCover struct {
								Urls []string `json:"urls"`
							} `json:"url_cover"`
						} `json:"album"`
						LabelInfo struct {
							OnlyVipDownload bool `json:"only_vip_download"`
						} `json:"label_info"`
						BitRates []struct {
							Size int64 `json:"size"`
						} `json:"bit_rates"`
					} `json:"track"`
				} `json:"entity"`
			} `json:"data"`
		} `json:"result_groups"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda search json parse error: %w", err)
	}

	if len(resp.ResultGroups) == 0 {
		return nil, nil
	}

	// 4. 转换模型
	var songs []model.Song
	for _, item := range resp.ResultGroups[0].Data {
		track := item.Entity.Track

		var artistNames []string
		for _, ar := range track.Artists {
			artistNames = append(artistNames, ar.Name)
		}

		var cover string
		if len(track.Album.UrlCover.Urls) > 0 {
			cover = track.Album.UrlCover.Urls[0]
		}

		var maxSize int64
		for _, br := range track.BitRates {
			if br.Size > maxSize {
				maxSize = br.Size
			}
		}

		songs = append(songs, model.Song{
			Source:   "soda",
			ID:       track.ID,
			Name:     track.Name,
			Artist:   strings.Join(artistNames, "、"),
			Album:    track.Album.Name,
			Duration: track.Duration / 1000,
			Size:     maxSize,
			Cover:    cover,
		})
	}

	return songs, nil
}

// DownloadInfo 包含下载所需的 URL 和 解密 Key
type DownloadInfo struct {
	URL      string // 加密的音频链接
	PlayAuth string // 解密 Key (Base64)
	Format   string // 文件格式 (m4a)
	Size     int64  // 文件大小
}

// GetDownloadInfo 获取下载信息 (URL + Auth)
func GetDownloadInfo(s *model.Song) (*DownloadInfo, error) {
	if s.Source != "soda" {
		return nil, errors.New("source mismatch")
	}

	params := url.Values{}
	params.Set("track_id", s.ID)
	params.Set("media_type", "track")

	v2URL := "https://api.qishui.com/luna/pc/track_v2?" + params.Encode()
	v2Body, err := utils.Get(v2URL, utils.WithHeader("User-Agent", UserAgent))
	if err != nil {
		return nil, err
	}

	var v2Resp struct {
		TrackPlayer struct {
			URLPlayerInfo string `json:"url_player_info"`
		} `json:"track_player"`
	}
	if err := json.Unmarshal(v2Body, &v2Resp); err != nil {
		return nil, err
	}

	if v2Resp.TrackPlayer.URLPlayerInfo == "" {
		return nil, errors.New("player info url not found")
	}

	infoBody, err := utils.Get(v2Resp.TrackPlayer.URLPlayerInfo, utils.WithHeader("User-Agent", UserAgent))
	if err != nil {
		return nil, err
	}

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

	if err := json.Unmarshal(infoBody, &infoResp); err != nil {
		return nil, err
	}

	list := infoResp.Result.Data.PlayInfoList
	if len(list) == 0 {
		return nil, errors.New("no audio stream found")
	}

	// 排序取最高音质
	sort.Slice(list, func(i, j int) bool {
		if list[i].Size != list[j].Size {
			return list[i].Size > list[j].Size
		}
		return list[i].Bitrate > list[j].Bitrate
	})

	best := list[0]
	downloadURL := best.MainPlayUrl
	if downloadURL == "" {
		downloadURL = best.BackupPlayUrl
	}

	if downloadURL == "" {
		return nil, errors.New("invalid download url")
	}

	return &DownloadInfo{
		URL:      downloadURL,
		PlayAuth: best.PlayAuth,
		Format:   best.Format,
		Size:     best.Size,
	}, nil
}

// GetDownloadURL 返回下载链接，附带 Auth Key
// 注意：下载后的文件必须使用 DecryptAudio 函数进行解密才能播放
func GetDownloadURL(s *model.Song) (string, error) {
	info, err := GetDownloadInfo(s)
	if err != nil {
		return "", err
	}
	return info.URL + "#auth=" + url.QueryEscape(info.PlayAuth), nil
}

// --------------------------------------------------------------------------------
// 以下为移植自 Python (sodautils.py) 的解密逻辑
// --------------------------------------------------------------------------------

// DecryptAudio 解密汽水音乐下载的加密音频数据
// fileData: 下载的原始文件字节数据
// playAuth: GetDownloadInfo 返回的 PlayAuth 字符串
func DecryptAudio(fileData []byte, playAuth string) ([]byte, error) {
	// 1. 提取 AES Key
	hexKey, err := extractKey(playAuth)
	if err != nil {
		return nil, fmt.Errorf("failed to extract key: %w", err)
	}
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("invalid hex key: %w", err)
	}

	// 2. 解析 MP4 Box 结构
	moov, err := findBox(fileData, "moov", 0, len(fileData))
	if err != nil {
		return nil, errors.New("moov box not found")
	}

	// 查找 track 相关 box
	trak, err := findBox(fileData, "trak", moov.offset+8, moov.offset+moov.size)
	if err != nil {
		return nil, errors.New("trak box not found")
	}
	mdia, err := findBox(fileData, "mdia", trak.offset+8, trak.offset+trak.size)
	if err != nil {
		return nil, errors.New("mdia box not found")
	}
	minf, err := findBox(fileData, "minf", mdia.offset+8, mdia.offset+mdia.size)
	if err != nil {
		return nil, errors.New("minf box not found")
	}
	stbl, err := findBox(fileData, "stbl", minf.offset+8, minf.offset+minf.size)
	if err != nil {
		return nil, errors.New("stbl box not found")
	}

	// 3. 获取 Sample Sizes (stsz)
	stsz, err := findBox(fileData, "stsz", stbl.offset+8, stbl.offset+stbl.size)
	if err != nil {
		return nil, errors.New("stsz box not found")
	}
	sampleSizes := parseStsz(stsz.data)

	// 4. 获取 Encryption Info (senc)
	// senc 可能在 moov 下，也可能在 stbl 下
	senc, err := findBox(fileData, "senc", moov.offset+8, moov.offset+moov.size)
	if err != nil {
		senc, err = findBox(fileData, "senc", stbl.offset+8, stbl.offset+stbl.size)
	}
	if err != nil {
		return nil, errors.New("senc box not found")
	}
	ivs := parseSenc(senc.data)

	// 5. 获取 Media Data (mdat)
	mdat, err := findBox(fileData, "mdat", 0, len(fileData))
	if err != nil {
		return nil, errors.New("mdat box not found")
	}

	// 6. 执行 AES-CTR 解密
	// 为了安全，创建一个新的缓冲区用于存储解密后的 mdat 数据
	decryptedMdat := make([]byte, 0, mdat.size-8)
	readPtr := mdat.offset + 8

	// 创建 Cipher Block
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(sampleSizes); i++ {
		size := int(sampleSizes[i])
		if readPtr+size > len(fileData) {
			break
		}
		chunk := fileData[readPtr : readPtr+size]

		if i < len(ivs) {
			// AES-CTR Decrypt
			// 注意：Python 代码中对每个 sample 都使用了特定的 IV
			stream := cipher.NewCTR(block, ivs[i])
			dst := make([]byte, size)
			stream.XORKeyStream(dst, chunk)
			decryptedMdat = append(decryptedMdat, dst...)
		} else {
			// 如果没有 IV (通常不会发生)，直接拷贝
			decryptedMdat = append(decryptedMdat, chunk...)
		}
		readPtr += size
	}

	// 7. 替换 stsd 中的 enca -> mp4a
	stsd, err := findBox(fileData, "stsd", stbl.offset+8, stbl.offset+stbl.size)
	if err == nil {
		// 原地替换字节
		stsdData := fileData[stsd.offset : stsd.offset+stsd.size]
		if idx := bytes.Index(stsdData, []byte("enca")); idx != -1 {
			copy(stsdData[idx:], []byte("mp4a"))
		}
	}

	// 8. 组装最终文件
	// 如果解密后的 mdat 大小一致，直接替换原文件中的 mdat 内容
	if len(decryptedMdat) == int(mdat.size)-8 {
		copy(fileData[mdat.offset+8:], decryptedMdat)
	} else {
		// 理论上大小应该一致
		return nil, errors.New("decrypted size mismatch")
	}

	return fileData, nil
}

// --- Helper Functions (SpadeDecryptor & AudioDecryptor) ---

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
			return &mp4Box{
				offset: pos,
				size:   size,
				data:   data[pos+8 : pos+size],
			}, nil
		}
		pos += size
	}
	return nil, errors.New("box not found")
}

func parseStsz(data []byte) []uint32 {
	// stsz structure: version(1) + flags(3) + sample_size(4) + sample_count(4) + entry_size(4)*count
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
	// senc structure: version(1) + flags(3) + sample_count(4) + (IV(8) + [subsamples])
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
		// IV is 8 bytes, need to pad to 16 bytes for AES-CTR
		rawIV := data[ptr : ptr+8]
		iv := make([]byte, 16)
		copy(iv, rawIV) // Pad with 0s
		ivs = append(ivs, iv)
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

// --- SpadeDecryptor Logic ---

func bitcount(n int) int {
	// 32-bit integer population count logic from python
	u := uint32(n)
	u = u & 0xFFFFFFFF
	u = u - ((u >> 1) & 0x55555555)
	u = (u & 0x33333333) + ((u >> 2) & 0x33333333)
	return int(((u + (u >> 4) & 0xF0F0F0F) * 0x1010101) >> 24)
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
		// Python: v = ...; while v < 0: v += 255
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
	decodedLen := len(bytesData) - paddingLen - 2
	endIndex := 1 + decodedLen - skipBytes

	if endIndex > len(tmpBuff) || 1 > endIndex {
		return "", errors.New("index out of bounds")
	}

	finalBytes := tmpBuff[1:endIndex]
	return string(finalBytes), nil
}