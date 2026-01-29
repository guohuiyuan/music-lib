# 🎵 music-lib: 你的 Go 音乐聚合搜索“中枢神经”

`music-lib` 是一个专为 Go 开发者打造的纯粹的音乐搜索与下载核心库。它被精心设计为一个高度可扩展的后端模块，旨在为你的应用提供一个稳定、统一且强大的音乐数据接口。你可以把它想象成一个处理所有与音乐平台对接的“中枢神经系统”。

## 核心设计

- **纯粹的后端引擎**: 我们只提供干净的函数接口，不掺杂任何 UI 逻辑，让你可以轻松地将其集成到任何 Go 项目中，无论是 Web 应用、桌面程序还是机器人。
- **多平台无缝聚合**: 支持 **网易云、QQ 音乐、酷狗** 等十多个主流音乐平台，让你免于为每个平台单独编写适配代码的烦恼。
- **标准化数据模型**: 无论音乐来自哪个平台，我们都将其“翻译”成统一的 `model.Song` 结构。这意味着你只需编写一次处理逻辑，就能适用于所有平台。
- **模块化架构**: 每个音乐平台都是一个独立的 `provider`（提供者）。这种设计不仅易于维护，更让添加对新平台的支持变得像搭积木一样简单。
- **智能音源过滤**: 自动跳过大多数需要 VIP 或付费的歌曲，优先为你筛选出可免费播放和下载的音源，让你的应用更具竞争力。
- **强大的高级功能**:
  - **歌词一键获取**: 为支持的平台提供统一的 `GetLyrics` 接口，轻松实现歌词功能。
  - **音频格式无忧**: 内置对**汽水音乐**等平台特殊加密音频的解密功能，让你的用户无需关心底层细节。

## 已支持的音乐平台

| 平台 | 模块名 | 搜索 | 下载链接 | 歌词 | 备注 |
| :--- | :--- | :---: | :---: | :---: | :--- |
| 网易云音乐 | `netease` | ✅ | ✅ | ✅ | |
| QQ 音乐 | `qq` | ✅ | ✅ | ✅ | |
| 酷狗音乐 | `kugou` | ✅ | ✅ | ✅ | |
| 酷我音乐 | `kuwo` | ✅ | ✅ | ✅ | |
| 咪咕音乐 | `migu` | ✅ | ✅ | ✅ | |
| 千千音乐 | `qianqian` | ✅ | ✅ | ✅ | |
| 汽水音乐 | `soda` | ✅ | ✅ | ✅ | 支持音频解密 |
| 5sing | `fivesing` | ✅ | ✅ | ✅ | |
| Jamendo | `jamendo` | ✅ | ✅ | ❌ | |
| JOOX | `joox` | ✅ | ✅ | ✅ | |
| Bilibili | `bilibili` | ✅ | ✅ | ✅ | |

## 快速集成

在你的项目中，通过 Go Modules 引入 `music-lib`:
```bash
go get github.com/guohuiyuan/music-lib
```

## 实战演练

下面的示例将带你领略 `music-lib` 的简洁与强大。我们将演示如何搜索歌曲并获取其下载链接。

```go
package main

import (
	"fmt"
	"log"

	"github.com/guohuiyuan/music-lib/kugou" // 以酷狗音乐为例，你可以轻松换成其他 provider
	"github.com/guohuiyuan/music-lib/model"
)

func main() {
	keyword := "周杰伦"

	// 1. 搜索歌曲
	// 每个 provider 包都提供了统一的 Search 函数
	songs, err := kugou.Search(keyword)
	if err != nil {
		log.Fatalf("搜索失败，错误: %v", err)
	}

	if len(songs) == 0 {
		fmt.Println("未找到相关歌曲")
		return
	}

	fmt.Printf("在酷狗音乐找到 %d 首关于 '%s' 的歌曲:\n", len(songs), keyword)

	// 2. 选择一首歌，获取下载链接
	// 这里我们以第一首歌为例
	firstSong := songs[0]
	fmt.Printf("正在为 '%s - %s' 获取下载链接...\n", firstSong.Name, firstSong.Artist)

	// GetDownloadURL 函数同样是每个 provider 的标配
	downloadURL, err := kugou.GetDownloadURL(&firstSong)
	if err != nil {
		log.Fatalf("获取下载链接失败，错误: %v", err)
	}

	fmt.Println("下载链接已到手:")
	fmt.Println(downloadURL)

	// 3. (可选) 获取歌词
	lyrics, err := kugou.GetLyrics(&firstSong)
	if err != nil {
		log.Printf("获取歌词失败，错误: %v", err)
	} else {
		fmt.Println("\n歌词 (LRC 格式):")
		fmt.Println(lyrics)
	}
}
```

## 架构透视

```
music-lib/
├── model/                # 通用数据结构 (例如，所有歌曲信息都装在 model.Song 这个“标准集装箱”里)
│   └── song.go
├── utils/                # 实用工具箱 (HTTP 请求、文件处理等)
│   ├── file.go
│   └── request.go
├── provider/             # 所有音乐平台的“行为准则” (接口定义)
│   └── interface.go
├── netease/              # 网易云音乐平台的具体实现
├── qq/                   # QQ 音乐平台的具体实现
├── kugou/                # 酷狗音乐平台的具体实现
...                       # 其他音乐平台，等待你的探索和贡献
├── go.mod
└── README.md
```

## 设计哲学

- **高内聚，低耦合**: 每个音乐平台包（`provider`）都是一个独立的王国，它们之间互不干涉。所有 `provider` 都遵循 `provider.Interface` 这份“共同宪法”，这使得上层应用可以“一视同仁”地对待来自不同平台的数据。
- **单一职责**: `music-lib` 的使命只有一个：获取和处理音乐数据。它不关心你的应用长什么样，也不干涉你如何存储数据。这种专注使它成为一个理想的“音乐中台”。
- **为扩展而生**: 添加一个新的音乐平台？非常简单。只需创建一个新的包，实现 `provider.Interface` 中定义的方法，然后你就拥有了一个新的音乐源。

## 许可证

本项目基于 [GNU Affero General Public License v3.0](https://github.com/guohuiyuan/music-lib/blob/main/LICENSE) 许可。

## 免责声明

本项目仅供个人学习和技术研究使用。在使用本库时，请务必遵守相关法律法规及各大音乐平台的用户协议。通过本库获取的任何资源，请在 24 小时内删除。我们倡导尊重和支持正版音乐。