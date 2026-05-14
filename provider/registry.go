package provider

import (
	"strings"
	"sync"

	"github.com/guohuiyuan/music-lib/apple"
	"github.com/guohuiyuan/music-lib/bilibili"
	"github.com/guohuiyuan/music-lib/fivesing"
	"github.com/guohuiyuan/music-lib/jamendo"
	"github.com/guohuiyuan/music-lib/joox"
	"github.com/guohuiyuan/music-lib/kugou"
	"github.com/guohuiyuan/music-lib/kuwo"
	"github.com/guohuiyuan/music-lib/migu"
	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/qianqian"
	"github.com/guohuiyuan/music-lib/qq"
	"github.com/guohuiyuan/music-lib/soda"
)

type MusicProviderList map[string]MusicProvider

var (
	registryMu           sync.RWMutex
	StdMusicProviderList = MusicProviderList{}
)

func init() {
	InitMusic("netease", "", func(cookie string) MusicProvider { return netease.New(cookie) }, StdMusicProviderList)
	InitMusic("qq", "", func(cookie string) MusicProvider { return qq.New(cookie) }, StdMusicProviderList)
	InitMusic("kugou", "", func(cookie string) MusicProvider { return kugou.New(cookie) }, StdMusicProviderList)
	InitMusic("kuwo", "", func(cookie string) MusicProvider { return kuwo.New(cookie) }, StdMusicProviderList)
	InitMusic("migu", "", func(cookie string) MusicProvider { return migu.New(cookie) }, StdMusicProviderList)
	InitMusic("bilibili", "", func(cookie string) MusicProvider { return bilibili.New(cookie) }, StdMusicProviderList)
	InitMusic("fivesing", "", func(cookie string) MusicProvider { return fivesing.New(cookie) }, StdMusicProviderList)
	InitMusic("jamendo", "", func(cookie string) MusicProvider { return jamendo.New(cookie) }, StdMusicProviderList)
	InitMusic("joox", "", func(cookie string) MusicProvider { return joox.New(cookie) }, StdMusicProviderList)
	InitMusic("qianqian", "", func(cookie string) MusicProvider { return qianqian.New(cookie) }, StdMusicProviderList)
	InitMusic("soda", "", func(cookie string) MusicProvider { return soda.New(cookie) }, StdMusicProviderList)
	InitMusic("apple", "", func(cookie string) MusicProvider { return apple.New(cookie) }, StdMusicProviderList)
}

func InitMusic(source string, cookie string, registry func(string) MusicProvider, mp MusicProviderList) {
	source = strings.TrimSpace(source)
	if source == "" || registry == nil {
		return
	}

	registryMu.Lock()
	defer registryMu.Unlock()
	mp[source] = registry(cookie)
}

func NewMusicProviderList() MusicProviderList {
	return make(MusicProviderList)
}

func (mp MusicProviderList) GetMP(source string) MusicProvider {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil
	}
	return mp[source]
}

func (mp MusicProviderList) IsImplementFullMusicProvider(source string) (FullMusicProvider, bool) {
	m := mp.GetMP(source)
	if m == nil {
		return nil, false
	}
	p, ok := m.(FullMusicProvider)
	return p, ok
}
func (mp MusicProviderList) IsImplementQRLoginProvider(source string) (QRLoginProvider, bool) {
	m := mp.GetMP(source)
	if m == nil {
		return nil, false
	}
	p, ok := m.(QRLoginProvider)
	return p, ok
}

func (mp MusicProviderList) IsImplementUserPlaylistProvider(source string) (UserPlaylistProvider, bool) {
	m := mp.GetMP(source)
	if m == nil {
		return nil, false
	}
	p, ok := m.(UserPlaylistProvider)
	return p, ok
}
