# music-lib

music-lib 是个用 Go 写的音乐库。

它没有 UI，主要帮你解决各个音乐平台的数据接口问题——不管是搜索、解析还是下载。如果你想自己写个音乐下载器或者播放器，用它正好。

## 主要功能

支持网易云、QQ、酷狗、酷我这些主流平台，也能搞定汽水音乐、5sing 这些。具体支持情况如下：

| 平台       | 包名         | 搜索 | 下载 | 歌词 | 歌曲解析 | 歌单搜索 | 歌单推荐 | 歌单歌曲 | 歌单链接解析 | 备注     |
| :--------- | :----------- | :--: | :--: | :--: | :------: | :------: | :------: | :------: | :----------: | :------- |
| 网易云音乐 | `netease`  |  ✅  |  ✅  |  ✅  |    ✅    |    ✅    |    ✅    |    ✅    |      ✅      |          |
| QQ 音乐    | `qq`       |  ✅  |  ✅  |  ✅  |    ✅    |    ✅    |    ✅    |    ✅    |      ✅      |          |
| 酷狗音乐   | `kugou`    |  ✅  |  ✅  |  ✅  |    ✅    |    ✅    |    ✅    |    ✅    |      ✅      |          |
| 酷我音乐   | `kuwo`     |  ✅  |  ✅  |  ✅  |    ✅    |    ✅    |    ✅    |    ✅    |      ✅      |          |
| 咪咕音乐   | `migu`     |  ✅  |  ✅  |  ✅  |    ❌    |    ✅    |    ❌    |    ❌    |      ❌      |          |
| 千千音乐   | `qianqian` |  ✅  |  ✅  |  ✅  |    ❌    |    ❌    |    ❌    |    ✅    |      ❌      |          |
| 汽水音乐   | `soda`     |  ✅  |  ✅  |  ✅  |    ✅    |    ✅    |    ❌    |    ✅    |      ✅      | 音频解密 |
| 5sing      | `fivesing` |  ✅  |  ✅  |  ✅  |    ✅    |    ✅    |    ❌    |    ✅    |      ✅      |          |
| Jamendo    | `jamendo`  |  ✅  |  ✅  |  ❌  |    ✅    |    ❌    |    ❌    |    ❌    |      ❌      |          |
| JOOX       | `joox`     |  ✅  |  ✅  |  ✅  |    ❌    |    ✅    |    ❌    |    ❌    |      ❌      |          |
| Bilibili   | `bilibili` |  ✅  |  ✅  |  ❌  |    ✅    |    ✅    |    ❌    |    ✅    |      ✅      |          |

## 怎么用

直接 `go get`：

```bash
go get github.com/guohuiyuan/music-lib
```

### 1. 搜歌 + 下载

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
		fmt.Println("没找到相关歌曲")
		return
	}

	// 拿第一首的下载地址
	url, err := kugou.GetDownloadURL(&songs[0])
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("下载地址:", url)
}
```

### 2. 获取推荐歌单 (新功能)

```go
package main

import (
	"fmt"
	"log"
	"github.com/guohuiyuan/music-lib/netease"
)

func main() {
	// 获取不需要登录的推荐歌单
	playlists, err := netease.GetRecommendedPlaylists()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("拿到 %d 个推荐歌单：\n", len(playlists))
	for _, p := range playlists {
		fmt.Printf("- %s (ID: %s)\n", p.Name, p.ID)
	}
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

## 设计的一点想法

做这个库的时候，我尽量保证了**独立性**和**统一性**。

- **独立性**：你可以只引 `netease` 包，别的包不会进去污染你的依赖。
- **统一性**：不管用哪个包，返回的 `Song` 和 `Playlist` 结构都是一样的，切换源的时候不用改业务逻辑。
- **扩展性**：如果要加新平台，照着 `provider` 接口实现一遍就行。

## 目录结构

```
music-lib/
├── model/      # 都在用的数据结构
├── provider/   # 接口定义
├── netease/    # 各个平台的实现
├── qq/
├── kugou/
...
└── README.md
```

## 许可证

本项目遵循 GNU Affero General Public License v3.0（AGPL-3.0）。详情见 [LICENSE](LICENSE)。

## 免责声明

这个库就是写着玩、学技术的。大家用的时候遵守一下法律法规，不要拿去商用。下载的资源 24 小时内删掉。
