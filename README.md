# music-lib

`music-lib` 是一个用 Go 编写的音乐平台能力库。

它不提供 UI，主要用来统一不同平台的歌曲、歌单、专辑搜索与解析能力，适合拿来做下载器、播放器、聚合工具或你自己的上层服务。

## 主要功能

支持网易云、QQ、酷狗、酷我这些主流平台，也能搞定汽水音乐、5sing 这些。具体支持情况如下：

| 平台       | 包名         | 搜索 | 下载 | 歌词 | 歌曲解析 | 歌单搜索 | 歌单推荐 | 歌单歌曲 | 歌单链接解析 | 专辑搜索 | 专辑歌曲 | 专辑链接解析 | 备注                                     |
| ---------- | ------------ | ---- | ---- | ---- | -------- | -------- | -------- | -------- | ------------ | -------- | -------- | ------------ | ---------------------------------------- |
| 网易云音乐 | `netease`    | ✅   | ✅   | ✅   | ✅       | ✅       | ✅       | ✅       | ✅           | ✅       | ✅       | ✅           | 支持 FLAC 无损                           |
| QQ 音乐    | `qq`         | ✅   | ✅   | ✅   | ✅       | ✅       | ✅       | ✅       | ✅           | ✅       | ✅       | ✅           | 支持 FLAC 无损                           |
| 酷狗音乐   | `kugou`      | ✅   | ✅   | ✅   | ✅       | ✅       | ✅       | ✅       | ✅           | ✅       | ✅       | ✅           | 支持普通歌曲 FLAC 无损                   |
| 酷我音乐   | `kuwo`       | ✅   | ✅   | ✅   | ✅       | ✅       | ✅       | ✅       | ✅           | ✅       | ✅       | ✅           |                                          |
| 咪咕音乐   | `migu`       | ✅   | ✅   | ✅   | ✅       | ✅       | ❌       | ✅       | ✅           | ✅       | ✅       | ✅           | 歌单歌曲使用 MIGUM3 接口                 |
| 千千音乐   | `qianqian`   | ✅   | ✅   | ✅   | ✅       | ⚠️       | ❌       | ✅       | ✅           | ✅       | ✅       | ✅           | 歌单搜索可能返回空，已知 ID/链接可解析   |
| 汽水音乐   | `soda`       | ✅   | ✅   | ✅   | ✅       | ✅       | ❌       | ✅       | ✅           | ✅       | ✅       | ✅           | 音频解密                                 |
| 5sing      | `fivesing`   | ✅   | ✅   | ✅   | ✅       | ✅       | ❌       | ✅       | ✅           | ❌       | ❌       | ❌           |                                          |
| Jamendo    | `jamendo`    | ✅   | ✅   | ✅   | ✅       | ⚠️       | ❌       | ✅       | ✅           | ✅       | ✅       | ✅           | 歌单搜索可能返回空，公开歌单链接可解析   |
| JOOX       | `joox`       | ✅   | ✅   | ✅   | ✅       | ✅       | ❌       | ✅       | ✅           | ✅       | ✅       | ✅           | 歌单支持 OpenJOOX 接口和网页数据兜底     |
| Bilibili   | `bilibili`   | ✅   | ✅   | ✅   | ✅       | ✅       | ❌       | ✅       | ✅           | ❌       | ❌       | ❌           | 支持 FLAC 无损                           |

> `⚠️` 表示方法已接入，但平台搜索接口结果不稳定；优先使用已知 ID 或链接解析。
> 本次从 `❌` 升级的能力已在 `go-music-dl` 侧通过集成测试验证；千千音乐和 Jamendo 的歌单搜索仍按不稳定能力标记。

## 安装

```bash
go get github.com/guohuiyuan/music-lib
```

## 使用示例

### 1. 搜歌并获取下载地址

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
		fmt.Println("没有找到相关歌曲")
		return
	}

	url, err := kugou.GetDownloadURL(&songs[0])
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("下载地址:", url)
}
```

### 2. 搜索专辑并获取专辑歌曲

```go
package main

import (
	"fmt"
	"log"

	"github.com/guohuiyuan/music-lib/qq"
)

func main() {
	albums, err := qq.SearchAlbum("Taylor")
	if err != nil {
		log.Fatal(err)
	}
	if len(albums) == 0 {
		fmt.Println("没有找到相关专辑")
		return
	}

	album := albums[0]
	songs, err := qq.GetAlbumSongs(album.ID)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("专辑: %s, 共 %d 首歌\n", album.Name, len(songs))
}
```

### 3. 解析歌单链接

```go
package main

import (
	"fmt"
	"log"

	"github.com/guohuiyuan/music-lib/netease"
)

func main() {
	link := "https://music.163.com/#/playlist?id=123456"
	playlist, songs, err := netease.ParsePlaylist(link)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s 共有 %d 首歌\n", playlist.Name, len(songs))
}
```

## 设计说明

- 独立性：每个平台包彼此独立，可以按需引入。
- 统一性：大部分平台统一返回 `Song` 和 `Playlist` 结构，便于上层复用。
- 可扩展性：新增平台时，按现有包结构补实现即可。

## 目录结构

```text
music-lib/
├── model/      # 通用数据结构
├── provider/   # 接口定义
├── netease/    # 各平台实现
├── qq/
├── kugou/
├── kuwo/
├── migu/
├── qianqian/
├── soda/
├── ...
└── README.md
```

## 许可证

本项目使用 `GNU Affero General Public License v3.0 (AGPL-3.0)`，详见 [LICENSE](LICENSE)。

## 免责声明

这个库就是写着玩、学技术的。大家用的时候遵守一下法律法规，不要拿去商用。下载的资源 24 小时内删掉。
