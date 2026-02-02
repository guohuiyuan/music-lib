# music-lib

music-lib 是一个 Go 音乐库，提供统一的搜索、解析与下载接口。它不带 UI，适合嵌进自己的工具里。

## 特性

- 多平台搜索与下载
- 统一的数据模型（`model.Song` / `model.Playlist`）
- 平台模块化，按需引入
- 歌词、歌单、链接解析
- 支持汽水音乐等加密音频
- 自动过滤部分付费资源

## 支持平台

| 平台 | 模块名 | 搜索 | 下载 | 歌词 | 链接解析 | 歌单搜索 | 歌单歌曲 | 备注 |
| :--- | :--- | :---: | :---: | :---: | :---: | :---: | :---: | :--- |
| 网易云音乐 | `netease` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | |
| QQ 音乐 | `qq` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | |
| 酷狗音乐 | `kugou` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | |
| 酷我音乐 | `kuwo` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | |
| 咪咕音乐 | `migu` | ✅ | ✅ | ✅ | ❌ | ✅ | ❌ | |
| 千千音乐 | `qianqian` | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ | |
| 汽水音乐 | `soda` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 支持音频解密 |
| 5sing | `fivesing` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | |
| Jamendo | `jamendo` | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ | |
| JOOX | `joox` | ✅ | ✅ | ✅ | ❌ | ✅ | ❌ | |
| Bilibili | `bilibili` | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ | |

## 安装

```bash
go get github.com/guohuiyuan/music-lib
```

## 示例

### 搜索 + 下载链接

```go
package main

import (
	"fmt"
	"log"

	"github.com/guohuiyuan/music-lib/kugou"
)

func main() {
	songs, err := kugou.Search("周杰伦")
	if err != nil {
		log.Fatal(err)
	}
	if len(songs) == 0 {
		fmt.Println("没有结果")
		return
	}
	url, err := kugou.GetDownloadURL(&songs[0])
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(url)
}
```

### 链接解析

```go
package main

import (
	"fmt"
	"log"

	"github.com/guohuiyuan/music-lib/netease"
)

func main() {
	link := "https://music.163.com/#/song?id=123456"
	song, err := netease.Parse(link)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s - %s\n", song.Artist, song.Name)
}
```

### 歌单搜索

```go
package main

import (
	"fmt"
	"log"

	"github.com/guohuiyuan/music-lib/netease"
)

func main() {
	playlists, err := netease.SearchPlaylist("经典老歌")
	if err != nil {
		log.Fatal(err)
	}
	if len(playlists) == 0 {
		fmt.Println("没有结果")
		return
	}
	songs, err := netease.GetPlaylistSongs(playlists[0].ID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("songs:", len(songs))
}
```

## 目录结构

```
music-lib/
├── model/
├── utils/
├── provider/
├── netease/
├── qq/
├── kugou/
├── kuwo/
├── migu/
├── soda/
├── bilibili/
├── fivesing/
├── jamendo/
├── joox/
├── qianqian/
└── README.md
```

## 数据结构要点

- `Song` 新增了 `Link`、`Extra`、`IsInvalid`
- `Playlist` 保留来源与原始链接字段

## 说明

该库不包含 UI 逻辑。具体的播放器、换源与缓存策略建议在上层应用里实现。

## 设计

- **高内聚，低耦合**: 每个音乐平台包独立，遵循统一接口
- **单一职责**: 专注于音乐数据获取和处理
- **易于扩展**: 添加新平台只需实现接口

## 使用示例

### 多平台并发搜索

```go
package main

import (
	"fmt"
	"sync"

	"github.com/guohuiyuan/music-lib/kugou"
	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/qq"
)

func main() {
	keyword := "晴天"

	var wg sync.WaitGroup
	var mu sync.Mutex
	var allSongs []model.Song

	// 并发搜索多个平台
	searchFuncs := []func(string) ([]model.Song, error){
		netease.Search,
		qq.Search,
		kugou.Search,
	}

	for _, search := range searchFuncs {
		wg.Add(1)
		go func(fn func(string) ([]model.Song, error)) {
			defer wg.Done()
			songs, err := fn(keyword)
			if err == nil && len(songs) > 0 {
				mu.Lock()
				allSongs = append(allSongs, songs...)
				mu.Unlock()
			}
		}(search)
	}

	wg.Wait()

	fmt.Printf("共找到 %d 首歌曲\n", len(allSongs))
	for _, song := range allSongs {
		fmt.Printf("- %s - %s (%s)\n", song.Artist, song.Name, song.Source)
	}
}
```

## 许可证

本项目基于 [CharlesPikachu/musicdl](https://github.com/CharlesPikachu/musicdl) 的核心设计思路开发，遵循 [PolyForm Noncommercial License 1.0.0](https://polyformproject.org/licenses/noncommercial/1.0.0) 协议，禁止任何商业使用。

## 免责声明

本项目仅供个人学习和技术研究使用。在使用本库时，请遵守相关法律法规及音乐平台用户协议。通过本库获取的资源，请在 24 小时内删除。
