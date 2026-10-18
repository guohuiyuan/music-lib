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
| 汽水音乐   | `soda`       | ✅   | ✅   | ✅   | ✅       | ✅       | ❌       | ✅       | ✅           | ✅       | ✅       | ✅           | 音频解密，支持短链、个人歌单；Cookie + 内置本机签名器（官方客户端 bdms.node，无常驻服务）可下会员整曲，也可配远程签名服务，都没有时为试听片段 |
| 5sing      | `fivesing`   | ✅   | ✅   | ✅   | ✅       | ✅       | ❌       | ✅       | ✅           | ❌       | ❌       | ❌           |                                          |
| Jamendo    | `jamendo`    | ✅   | ✅   | ✅   | ✅       | ⚠️       | ❌       | ✅       | ✅           | ✅       | ✅       | ✅           | 歌单搜索可能返回空，公开歌单链接可解析   |
| JOOX       | `joox`       | ✅   | ✅   | ✅   | ✅       | ✅       | ❌       | ✅       | ✅           | ✅       | ✅       | ✅           | 歌单支持 OpenJOOX 接口和网页数据兜底     |
| Bilibili   | `bilibili`   | ✅   | ✅   | ✅   | ✅       | ✅       | ❌       | ✅       | ✅           | ❌       | ❌       | ❌           | 支持 FLAC 无损                           |
| Apple Music | `apple`     | ✅   | ⚠️  | ✅   | ✅       | ✅       | ❌       | ✅       | ✅           | ✅       | ✅       | ✅           | 下载仅 preview，完整需 gamdl 解密        |

> `⚠️` 表示方法已接入，但平台搜索接口结果不稳定；优先使用已知 ID 或链接解析。
> 本次从 `❌` 升级的能力已在 `go-music-dl` 侧通过集成测试验证；千千音乐和 Jamendo 的歌单搜索仍按不稳定能力标记。

个人化、登录和歌单分类能力支持情况：

| 平台       | 个人歌单 | 扫码登录 | 歌单分类 |
| ---------- | -------- | -------- | -------- |
| 网易云音乐 | ✅       | ✅       | ✅       |
| QQ 音乐    | ✅       | ✅ QQ / 微信 | ✅       |
| 酷狗音乐   | ✅       | ✅       | ✅       |
| 酷我音乐   | ❌       | ❌       | ✅       |
| 咪咕音乐   | ❌       | ❌       | ✅       |
| 千千音乐   | ❌       | ❌       | ✅       |
| 汽水音乐   | ✅       | ⚠️ 未调通 | ✅       |
| 5sing      | ❌       | ❌       | ❌       |
| Jamendo    | ❌       | ❌       | ❌       |
| JOOX       | ❌       | ❌       | ✅       |
| Bilibili   | ❌       | ✅       | ❌       |
| Apple Music | ❌      | ❌       | ✅       |

排行榜与热门单曲能力支持情况（`provider.TopListProvider` / `provider.HotSongProvider`）：

| 平台       | 排行榜列表 | 榜单歌曲 | 热门单曲 | 备注                                                      |
| ---------- | ---------- | -------- | -------- | --------------------------------------------------------- |
| 网易云音乐 | ✅         | ✅       | ✅       | 支持全部榜单，热门单曲取“热歌榜”，无需登录，支持 Cookie    |
| QQ 音乐    | ✅         | ✅       | ✅       | 支持巅峰榜/地区榜/特色榜/全球榜共 29 个榜单，热门单曲取“热歌榜”（topId=26），无需登录，支持 Cookie |
| 酷狗音乐   | ✅         | ✅       | ✅       | 支持热门榜/地区榜/特色榜/全球榜/曲风榜共 56 个榜单，热门单曲取“TOP500”（rankid=8888），无需登录，支持 Cookie |
| 酷我音乐   | ✅         | ✅       | ✅       | 支持官方/特色/场景/语言/全球共 36 个榜单，热门单曲取“酷我热歌榜”（id=16），无需登录，支持 Cookie；榜单歌曲接口不带封面 |
| 咪咕音乐   | ✅         | ✅       | ✅       | 支持咪咕特色榜/咪咕尖叫榜/精选榜单共 18 个榜单（移动端目录失败时回退 PC 端 9 个榜单），热门单曲取“全网热歌榜”（rankId=83048887），无需登录，支持 Cookie |
| 千千音乐   | ❌         | ❌       | ❌       | 未支持，返回 `model.ErrTopListsUnsupported`               |
| 汽水音乐   | ✅         | ✅       | ✅       | 内置热歌榜/新歌榜/欧美榜/音乐人歌曲榜共 4 个官方榜单，并聚合发现页推荐歌单（接口不可用时自动降级），热门单曲取“热歌榜”（chart_id=7036274230471712007），无需登录，支持 Cookie |
| 5sing      | ❌         | ❌       | ❌       | 未支持，返回 `model.ErrTopListsUnsupported`               |
| Jamendo    | ❌         | ❌       | ❌       | 未支持，返回 `model.ErrTopListsUnsupported`               |
| JOOX       | ❌         | ❌       | ❌       | 未支持，返回 `model.ErrTopListsUnsupported`               |
| Bilibili   | ❌         | ❌       | ❌       | 未支持，返回 `model.ErrTopListsUnsupported`               |
| Apple Music | ✅        | ✅       | ✅       | 支持中国内地/中国香港/中国台湾/美国/日本/韩国/英国/新加坡共 8 个官方 Top 100 歌曲榜（走 Apple 营销工具榜单接口，无需登录与 token），热门单曲取当前实例商店（默认 us）的 Top 100，试听地址由 iTunes 公开接口补全 |

未支持热门单曲的平台返回 `model.ErrHotSongsUnsupported`。

## Soda 扫码登录

`soda.CreateQRLogin` / `soda.CheckQRLogin` 目前仍处于调试状态。汽水音乐新版 PC 官方 passport 扫码流程依赖动态 `a_bogus` / `msToken` 风控签名，当前实现尚未调通完整登录链路，调用方不要把它作为稳定扫码登录能力暴露给用户。

已尝试按抓包还原 `check_qrconnect`、`send_code`、`validate_code`、MFA 后二次 `check_qrconnect` 的请求体和字段顺序，但真实轮询仍可能在 `new` / `scanned` 阶段触发 `error_code=7`。如果后续能实时生成官方签名，再恢复稳定支持。

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

### 4. 获取个人歌单

个人歌单需要登录 Cookie。支持的平台可以使用 `New(cookie)` 创建实例，再调用 `GetUserPlaylists`。目前支持网易云音乐、QQ 音乐、酷狗音乐和汽水音乐。

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/guohuiyuan/music-lib/qq"
)

func main() {
	q := qq.New(os.Getenv("QQ_COOKIE"))

	playlists, err := q.GetUserPlaylists(1, 20)
	if err != nil {
		log.Fatal(err)
	}
	if len(playlists) == 0 {
		fmt.Println("没有个人歌单")
		return
	}

	playlist := playlists[0]
	songs, err := q.GetPlaylistSongs(playlist.ID)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("歌单: %s, 共 %d 首歌\n", playlist.Name, len(songs))
}
```

QQ 音乐个人歌单里可能出现一些内部 ID：

- `profile:favorites`：我喜欢的歌曲。
- `profile:dir:<dirid>`：微信登录用户的个人目录歌单。

这些 ID 可以直接传给 `GetPlaylistSongs`，不需要自己转换成公开歌单 ID。

### 5. 扫码登录

已支持网易云、QQ、QQ 微信、酷狗、Bilibili 的扫码登录。登录成功后会返回 Cookie，建议由上层应用保存到配置或环境变量，不要硬编码在代码里。汽水音乐扫码登录暂未调通，请先手动获取 Cookie。

下面是 QQ 音乐微信扫码登录示例：

```go
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/qq"
)

func main() {
	session, err := qq.CreateWXQRLogin()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("请用微信扫描二维码:", session.ImageURL)

	for {
		time.Sleep(2 * time.Second)

		result, err := qq.CheckWXQRLogin(session.Key)
		if err != nil {
			log.Fatal(err)
		}

		switch result.Status {
		case model.QRLoginStatusWaiting, model.QRLoginStatusScanned:
			fmt.Println("登录状态:", result.Status)
		case model.QRLoginStatusSuccess:
			fmt.Println("登录成功，Cookie 长度:", len(result.Cookie))
			return
		case model.QRLoginStatusExpired, model.QRLoginStatusFailed:
			log.Fatalf("登录失败: %s", result.Message)
		}
	}
}
```

QQ 音乐默认扫码登录使用 `qq.CreateQRLogin` / `qq.CheckQRLogin`，微信扫码使用 `qq.CreateWXQRLogin` / `qq.CheckWXQRLogin`。如果想按类型切换，也可以使用 `qq.CreateQRLoginByType("qq")` / `qq.CheckQRLoginByType("qq", key)`，或 `qq.CreateQRLoginByType("wx")` / `qq.CheckQRLoginByType("wx", key)`。

### 6. 获取歌单分类

支持歌单分类的平台可以先获取分类，再按分类分页拉取歌单。

```go
package main

import (
	"fmt"
	"log"

	"github.com/guohuiyuan/music-lib/qq"
)

func main() {
	categories, err := qq.GetPlaylistCategories()
	if err != nil {
		log.Fatal(err)
	}
	if len(categories) == 0 {
		fmt.Println("没有分类")
		return
	}

	category := categories[0]
	playlists, err := qq.GetCategoryPlaylists(category.ID, 1, 20)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("分类: %s, 歌单数: %d\n", category.Name, len(playlists))
}
```

汽水音乐（`soda.GetPlaylistCategories` / `soda.GetCategoryPlaylists`）的分类来自发现页推荐歌单，
内置每日推荐、流行、华语、欧美、国风、民谣、摇滚、说唱、电子、R&B、治愈、睡前、学习 13 个分类，
分类 ID 即接口的 `sub_channel_id`；发现页接口没有游标，库内会并发抓取多轮并去重，因此 `page` / `limit` 仍然可用。

### 7. 获取排行榜与热门单曲

目前网易云音乐、QQ 音乐、酷狗音乐、酷我音乐、咪咕音乐、汽水音乐、Apple Music 实现了排行榜（`provider.TopListProvider`）和热门单曲（`provider.HotSongProvider`）能力，其余平台调用会返回 `model.ErrTopListsUnsupported` / `model.ErrHotSongsUnsupported`。这些平台的榜单接口都无需登录，带上 Cookie 也可以正常调用。汽水音乐没有公开的榜单目录接口，`GetTopLists()` 返回内置的 4 个官方榜单，并额外聚合发现页的推荐歌单（`toplist_type=playlist`，接口不可用时只返回官方榜单）；`GetTopListSongs()` 对官方榜单走 charts 接口、对推荐歌单走歌单详情接口，自动识别。Apple Music 的榜单目录同样是内置的，`GetTopLists()` 返回各国家/地区的 Top 100 歌曲榜，`GetTopListSongs()` 走 Apple 营销工具的 `most-played` 榜单接口（无需 token，遇到 5xx 会自动重试并切换备用域名），再用 iTunes 公开查询补全专辑、时长与试听地址。

```go
package main

import (
	"fmt"
	"log"

	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/qq"
)

func main() {
	// 榜单目录：飙升榜、热歌榜、新歌榜……
	topLists, err := netease.GetTopLists()
	if err != nil {
		log.Fatal(err)
	}
	for _, topList := range topLists {
		fmt.Printf("%s(id=%s) 更新: %s, 歌曲数: %d\n",
			topList.Name, topList.ID, topList.UpdateFrequency, topList.TrackCount)
	}

	// 榜单歌曲按名次排序，名次存放在歌曲 Extra["rank"]
	songs, err := netease.GetTopListSongs("19723756") // 飙升榜
	if err != nil {
		log.Fatal(err)
	}
	for _, song := range songs {
		fmt.Printf("#%s %s - %s\n", song.Extra["rank"], song.Name, song.Artist)
	}

	// 热门单曲 = 热歌榜，limit <= 0 时默认 100 首
	hotSongs, err := netease.GetHotSongs(20)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("热门单曲数:", len(hotSongs))
}
```

QQ 音乐的调用方式完全一致，榜单 ID 为 QQ 官方的 topId 字符串：

```go
topLists, err := qq.GetTopLists()          // 巅峰榜/地区榜/特色榜/全球榜
songs, err := qq.GetTopListSongs("26")     // 26 = 热歌榜
hotSongs, err := qq.GetHotSongs(20)        // 等价于热歌榜前 20 首
```

酷狗音乐的调用方式同样一致，榜单 ID 为酷狗官方的 rankid 字符串：

```go
import "github.com/guohuiyuan/music-lib/kugou"

topLists, err := kugou.GetTopLists()        // 热门榜/地区榜/特色榜/全球榜/曲风榜
songs, err := kugou.GetTopListSongs("8888") // 8888 = TOP500，共 500 首
hotSongs, err := kugou.GetHotSongs(20)      // 等价于 TOP500 前 20 首
```

酷我音乐同样一致，榜单 ID 为酷我榜单接口的 `sourceid` 字符串：

```go
import "github.com/guohuiyuan/music-lib/kuwo"

topLists, err := kuwo.GetTopLists()        // 官方/特色/场景/语言/全球
songs, err := kuwo.GetTopListSongs("16")   // 16 = 酷我热歌榜，共 300 首
hotSongs, err := kuwo.GetHotSongs(20)      // 等价于热歌榜前 20 首
```

咪咕音乐同样一致，榜单 ID 为咪咕榜单接口的 `rankId` 字符串；目录接口会按 `rankId` 全局去重：

```go
import "github.com/guohuiyuan/music-lib/migu"

topLists, err := migu.GetTopLists()             // 咪咕特色榜/咪咕尖叫榜/精选榜单，共 18 个
songs, err := migu.GetTopListSongs("83048887")  // 83048887 = 全网热歌榜，共 50 首
hotSongs, err := migu.GetHotSongs(20)           // 等价于全网热歌榜前 20 首
```

汽水音乐内置了 4 个官方榜单，榜单 ID 为汽水榜单的 `chart_id` 字符串：

```go
import "github.com/guohuiyuan/music-lib/soda"

topLists, err := soda.GetTopLists()   // 热歌榜/新歌榜/欧美榜/音乐人歌曲榜
songs, err := soda.GetTopListSongs("7036274230471712007") // 热歌榜，共 50 首
hotSongs, err := soda.GetHotSongs(20) // 等价于热歌榜前 20 首
```

Apple Music 的榜单 ID 形如 `most-played:songs:<storefront>`，也支持简写 `songs:cn` 或直接传商店代码 `cn`：

```go
import "github.com/guohuiyuan/music-lib/apple"

topLists, err := apple.GetTopLists()                     // 8 个国家的 Top 100 歌曲榜
songs, err := apple.GetTopListSongs("most-played:songs:cn") // 中国内地 Top 100，共 100 首
hotSongs, err := apple.GetHotSongs(20)                   // 默认 us 商店的 Top 100 前 20 首

cn := apple.New("storefront=cn")
hotSongs, err = cn.GetHotSongs(20)                       // 切换成中国内地 Top 100
```

Apple Music 的榜单歌曲自带 30 秒试听地址（`Song.URL`），完整歌曲仍需订阅与 gamdl 解密。

需要登录态时可以创建带 Cookie 的实例，行为与包级默认实例一致：

```go
n := netease.New(cookie) // cookie 从配置或环境变量读取
topLists, err := n.GetTopLists()
hotSongs, err := n.GetHotSongs(50)
```

### 8. 汽水音乐整曲下载（Cookie + 本机签名器）

汽水会员曲目的整曲流走 App 端点 `POST /luna/pc/track_v2`，请求必须带官方客户端的逐请求签名头（`x-helios` / `x-medusa`）；缺签名时该接口会返回空响应，公开的 SEO 端点无论是否登录都只给试听片段。

签名只能由官方原生组件生成（客户端安装目录里的 `bdms.node`，底层 `mssdk/metasecml.dll`），没有可离线复刻的纯软件实现。因此 `soda` 包内置了一个「本机签名器」：把一段随包编译进去的 JS 垫片交给 JS 运行时执行，直接调用 `bdms.node` 的 `init` + `generateHttpSignatureHeaders`，随用随起、不占端口、不需要任何常驻服务（参考 [sodam](https://github.com/sodahub-org/sodam) 的 mssdk 桥接协议，AGPL-3.0）。

```go
import (
	"log"

	"github.com/guohuiyuan/music-lib/soda"
)

// soda.New 默认就会探测本机签名器：只要本机装了汽水官方客户端（或系统里有 Node.js）即可用。
s := soda.New(cookie)

// 也可以显式声明（零值 = 自动探测运行时、客户端目录与身份缓存）：
s = soda.NewWithOptions(cookie, soda.WithLocalSigner(soda.LocalSignerOptions{}))

// 客户端不在默认位置时，直接传入包含 SodaMusic.exe 和 bdms.node 的版本目录：
opts := soda.LocalSignerOptions{
	ClientDir: `D:\Apps\Soda Music\3.1.0`,
}
s = soda.NewWithOptions(cookie, soda.WithLocalSigner(opts))

// 设备身份（device_id / iid / fp）首次签名时生成并缓存到
// <用户配置目录>/music-lib/soda-identity.json，可用 QISHUI_DEVICE_ID 固定。
soda.LocalSignerAvailable()                      // 使用默认目录探测
soda.LocalSignerAvailableWithOptions(opts)       // 使用自定义目录探测

// 换源/解析得到汽水歌曲后：
info, err := s.GetDownloadInfo(song) // info.Preview 为 true 表示拿到的是试听片段
if err != nil {
	log.Fatal(err)
}
if err := s.Download(song, "out.m4a"); err != nil {
	log.Fatal(err)
}
```

本机签名器相关环境变量：

| 变量 | 作用 |
| --- | --- |
| `QISHUI_LOCAL_SIGNER` | 开关：`0/off/false/no` 关闭，`1/on/true/yes` 强制开启，留空自动探测 |
| `QISHUI_NODE` | 指定 JS 运行时（默认依次找 `PATH` 里的 `node`、官方客户端自带的 Electron） |
| `QISHUI_CLIENT_DIR` / `QISHUI_BDMS_PATH` | 显式指定客户端安装目录 / `bdms.node` 路径 |
| `QISHUI_DEVICE_ID` | 固定设备 did（默认生成并缓存） |
| `QISHUI_IDENTITY_FILE` | 设备身份缓存文件路径 |

如果要把签名能力放到另一台装了客户端的机器上（远程签名服务，协议见 sodam），仍然支持：

```go
// 方式一：显式传入签名服务地址与 token（优先级高于本机签名器）
s = soda.NewWithSigner(cookie, "http://127.0.0.1:8899/sign", "token")

// 方式二：走环境变量（soda.New 会自动读取）
// QISHUI_SIGNER_URL=http://127.0.0.1:8899/sign
// QISHUI_SIGNER_TOKEN=token
s = soda.New(cookie)
```

远程签名服务两个端点即可：

- `POST /sign`：请求体 `{"url":"...","method":"POST","body":"...","ts_ms":1700000000000,"headers":{...}}`（headers 含 Cookie 与 `X-SS-STUB` = 请求体 MD5 大写），返回 `{"ok":true,"headers":{"X-Helios":"...","X-Medusa":"..."}}`。
- `GET /config`：返回设备身份（device_id / iid / fp），首次签名时自动获取并缓存。

本机和远程都不可用时的行为与旧版一致：免费歌曲取公开整曲，会员曲目回退到平台允许的试听片段（`DownloadInfo.Preview=true`）。远程签名器会看到完整请求头（含 Cookie），请只指向自己信任的实例。

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
