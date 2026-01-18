# Music Library

一个专业的音乐搜索和下载库，遵循高内聚、低耦合原则，可作为第三方库供其他项目引用。

## 特性

- **纯函数式接口**: 只负责输入参数 -> 返回数据，不包含UI逻辑
- **模块化设计**: 每个音乐源独立为一个包
- **标准化接口**: 使用通用的 `model.Song` 结构体
- **易于测试**: 没有 `fmt.Println` 和用户交互，易于编写单元测试
- **VIP过滤**: 自动过滤VIP和付费歌曲，仅返回免费可下载的歌曲

## 项目结构

```
music-lib/
├── go.mod
├── model/                # 通用数据结构
│   └── song.go
├── utils/                # 基础工具 (HTTP Client, Md5)
│   └── request.go
├── kugou/                # 酷狗源实现 (纯逻辑，无UI)
│   └── kugou.go
├── qq/                   # QQ音乐源实现 (基于旧版API)
│   └── qq.go
├── migu/                 # 咪咕音乐源实现 (基于安卓API)
│   └── migu.go
└── netease/              # 网易云音乐源实现 (基于Linux API和WeApi)
    ├── crypto.go         # 加密算法 (AES-ECB, AES-CBC, RSA)
    └── netease.go        # 核心业务逻辑
├── kuwo/                 # 酷我音乐源实现 (基于车载API)
    └── kuwo.go           # 核心业务逻辑
├── bilibili/             # Bilibili 音频源实现 (基于DASH API)
    └── bilibili.go       # 核心业务逻辑
```

## 安装

```bash
go get github.com/guohuiyuan/music-lib
```

## 使用示例

### 基本使用

```go
package main

import (
	"fmt"
	"log"
	
	"github.com/guohuiyuan/music-lib/kugou"
)

func main() {
	keyword := "周杰伦"
	
	// 1. 搜索歌曲
	songs, err := kugou.Search(keyword)
	if err != nil {
		log.Fatalf("搜索失败: %v", err)
	}

	fmt.Printf("找到 %d 首歌\n", len(songs))
	
	// 2. 显示搜索结果
	for i, song := range songs {
		if i >= 3 {
			break
		}
		fmt.Printf("%d. %s - %s (时长: %d秒)\n", i+1, song.Name, song.Artist, song.Duration)
	}

	// 3. 获取下载链接
	if len(songs) > 0 {
		target := &songs[0]
		url, err := kugou.GetDownloadURL(target)
		if err != nil {
			log.Printf("获取链接失败: %v", err)
		} else {
			fmt.Println("下载地址:", url)
		}
	}
}
```

### 在其他项目中引用

如果你在本地开发，需要在 `go.mod` 中添加 replace 指令：

```go
// 使用者的 go.mod
module my-app
go 1.25

require github.com/guohuiyuan/music-lib v1.0.0

```

## API 文档

### kugou 包

#### `func Search(keyword string) ([]model.Song, error)`
搜索酷狗音乐，返回歌曲列表。使用移动端 API (`songsearch.kugou.com`)，自动选择最佳音质（优先无损 SQFileHash，其次高品 HQFileHash，最后普通 FileHash）。**包含 `PayType` 和 `Privilege` 字段用于VIP过滤**（当前版本暂未启用过滤，因字段值含义待确认）。

#### `func GetDownloadURL(s *model.Song) (string, error)`
获取单首歌曲的下载地址。使用移动端 API (`m.kugou.com`)，需要正确的 User-Agent 和 Referer Header。注意：某些歌曲可能需要VIP权限。

### qq 包

#### `func Search(keyword string) ([]model.Song, error)`
搜索 QQ 音乐，返回歌曲列表。使用 `c.y.qq.com` API，支持多歌手拼接和歌曲时长信息。**自动过滤VIP和付费歌曲**（根据 `pay` 对象的 `pay_down` 和 `price_track` 字段）。

#### `func GetDownloadURL(s *model.Song) (string, error)`
获取单首歌曲的下载地址。使用 QQ 音乐的统一接口 `u.y.qq.com/cgi-bin/musicu.fcg`，通过 POST 请求获取 vkey。支持 128k MP3 和 M4A 音质。注意：某些热门歌曲可能需要 VIP 权限。

### migu 包

#### `func Search(keyword string) ([]model.Song, error)`
搜索咪咕音乐，返回歌曲列表。使用安卓客户端 API (`pd.musicapp.migu.cn`)，自动选择最佳音质（按文件大小降序排序）。返回的歌曲 ID 是复合格式：`ContentID|ResourceType|FormatType`。

#### `func GetDownloadURL(s *model.Song) (string, error)`
获取单首歌曲的下载地址。使用咪咕音乐的 API (`app.pd.nf.migu.cn`)，需要硬编码的 UserID。注意：返回的是 API 地址，访问时会重定向到实际文件。

### netease 包

#### `func Search(keyword string) ([]model.Song, error)`
搜索网易云音乐，返回歌曲列表。使用 Linux API (`api/linux/forward`)，参数经过 AES-ECB 加密。支持多歌手拼接和歌曲时长信息。**自动过滤VIP和付费歌曲**（根据 `fee` 和 `privilege` 字段）。

#### `func GetDownloadURL(s *model.Song) (string, error)`
获取单首歌曲的下载地址。使用 WeApi (`weapi/song/enhance/player/url`)，参数经过双重 AES-CBC 加密和 RSA 加密。支持 320kbps 码率。

### kuwo 包

#### `func Search(keyword string) ([]model.Song, error)`
搜索酷我音乐，返回歌曲列表。使用 `www.kuwo.cn` API，支持多歌手拼接和歌曲时长信息。注意：API 返回的 `DURATION` 字段是字符串类型，已自动转换为整数。**自动过滤VIP和付费歌曲**（根据 `pay` 字段，过滤包含 "pay" 且非 "0" 的条目）。

#### `func GetDownloadURL(s *model.Song) (string, error)`
获取单首歌曲的下载地址。使用车载客户端 API (`mobi.kuwo.cn`)，模拟车载客户端 (`source=kwplayercar_ar_6.0.0.9_B_jiakong_vh.apk`)。支持从高到低音质轮询：2000kflac (Hi-Res)、flac (无损)、320kmp3、192kmp3、128kmp3。

#### `func GetLyrics(s *model.Song) (string, error)`
获取歌词，返回 LRC 格式的歌词字符串。使用 `m.kuwo.cn` API，支持时间轴转换。

### bilibili 包

#### `func Search(keyword string) ([]model.Song, error)`
搜索 Bilibili 视频并展开为分 P 音频。使用 `api.bilibili.com/x/web-interface/search/type` API 搜索视频，然后为每个视频调用 `view` 接口获取分 P 列表。返回的歌曲 ID 是复合格式：`BVID|CID`。

#### `func GetDownloadURL(s *model.Song) (string, error)`
获取音频流链接。使用 DASH 格式 API (`api.bilibili.com/x/player/playurl`)，参数 `fnval=16` 请求音视频分离。支持音质优先级：Flac (无损) > Dolby (杜比) > Audio (普通)。注意：需要正确的 Cookie 才能获取高音质音频。

### model 包

#### `type Song struct`
通用的歌曲结构体，包含以下字段：
- `ID`: 歌曲ID
- `Name`: 歌曲名称
- `Artist`: 艺术家
- `Album`: 专辑名称
- `AlbumID`: 专辑ID
- `Duration`: 时长（秒）
- `Source`: 来源（kugou, netease, qq）
- `URL`: 下载地址
- `Size`: 文件大小

#### `func (s *Song) Display() string`
返回格式化的歌曲显示字符串。

### utils 包

#### `func Get(url string, opts ...RequestOption) ([]byte, error)`
发送HTTP GET请求，支持自定义Header。

#### `func MD5(str string) string`
计算字符串的MD5哈希值。

## 设计原则

1. **高内聚**: 每个包只负责一个明确的功能
2. **低耦合**: 包之间通过标准接口通信，减少依赖
3. **可测试性**: 纯函数设计，易于单元测试
4. **可扩展性**: 新的音乐源只需实现相同的接口即可集成

## 注意事项

- 某些歌曲可能需要VIP权限才能获取下载链接
- API可能会变更，需要定期维护
- 仅供学习和研究使用，请遵守相关法律法规

## 许可证

GNU General Public License v3.0
