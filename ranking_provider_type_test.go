package main

import (
	"errors"
	"testing"

	"github.com/guohuiyuan/music-lib/apple"
	"github.com/guohuiyuan/music-lib/bilibili"
	"github.com/guohuiyuan/music-lib/fivesing"
	"github.com/guohuiyuan/music-lib/jamendo"
	"github.com/guohuiyuan/music-lib/joox"
	"github.com/guohuiyuan/music-lib/kugou"
	"github.com/guohuiyuan/music-lib/kuwo"
	"github.com/guohuiyuan/music-lib/migu"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/provider"
	"github.com/guohuiyuan/music-lib/qianqian"
	"github.com/guohuiyuan/music-lib/qq"
	"github.com/guohuiyuan/music-lib/soda"
)

var _ provider.TopListProvider = (*netease.Netease)(nil)
var _ provider.TopListProvider = (*qq.QQ)(nil)
var _ provider.TopListProvider = (*kugou.Kugou)(nil)
var _ provider.TopListProvider = (*kuwo.Kuwo)(nil)
var _ provider.TopListProvider = (*migu.Migu)(nil)
var _ provider.TopListProvider = (*qianqian.Qianqian)(nil)
var _ provider.TopListProvider = (*soda.Soda)(nil)
var _ provider.TopListProvider = (*fivesing.Fivesing)(nil)
var _ provider.TopListProvider = (*jamendo.Jamendo)(nil)
var _ provider.TopListProvider = (*joox.Joox)(nil)
var _ provider.TopListProvider = (*bilibili.Bilibili)(nil)
var _ provider.TopListProvider = (*apple.Apple)(nil)

var _ provider.HotSongProvider = (*netease.Netease)(nil)
var _ provider.HotSongProvider = (*qq.QQ)(nil)
var _ provider.HotSongProvider = (*kugou.Kugou)(nil)
var _ provider.HotSongProvider = (*kuwo.Kuwo)(nil)
var _ provider.HotSongProvider = (*migu.Migu)(nil)
var _ provider.HotSongProvider = (*qianqian.Qianqian)(nil)
var _ provider.HotSongProvider = (*soda.Soda)(nil)
var _ provider.HotSongProvider = (*fivesing.Fivesing)(nil)
var _ provider.HotSongProvider = (*jamendo.Jamendo)(nil)
var _ provider.HotSongProvider = (*joox.Joox)(nil)
var _ provider.HotSongProvider = (*bilibili.Bilibili)(nil)
var _ provider.HotSongProvider = (*apple.Apple)(nil)

var _ provider.RankingProvider = (*netease.Netease)(nil)
var _ provider.RankingProvider = (*qq.QQ)(nil)
var _ provider.RankingProvider = (*kugou.Kugou)(nil)
var _ provider.RankingProvider = (*kuwo.Kuwo)(nil)
var _ provider.RankingProvider = (*migu.Migu)(nil)
var _ provider.RankingProvider = (*qianqian.Qianqian)(nil)
var _ provider.RankingProvider = (*soda.Soda)(nil)
var _ provider.RankingProvider = (*fivesing.Fivesing)(nil)
var _ provider.RankingProvider = (*jamendo.Jamendo)(nil)
var _ provider.RankingProvider = (*joox.Joox)(nil)
var _ provider.RankingProvider = (*bilibili.Bilibili)(nil)
var _ provider.RankingProvider = (*apple.Apple)(nil)

func TestRankingProviderUnsupported(t *testing.T) {
	// 未实现排行榜能力的平台应返回明确的 Unsupported 错误（这里以千千音乐为例）。
	if lists, err := qianqian.New("").GetTopLists(); !errors.Is(err, model.ErrTopListsUnsupported) || lists != nil {
		t.Fatalf("qianqian GetTopLists error mismatch: lists=%v err=%v", lists, err)
	}
	if songs, err := qianqian.New("").GetTopListSongs("1"); !errors.Is(err, model.ErrTopListsUnsupported) || songs != nil {
		t.Fatalf("qianqian GetTopListSongs error mismatch: songs=%v err=%v", songs, err)
	}
	if songs, err := qianqian.New("").GetHotSongs(10); !errors.Is(err, model.ErrHotSongsUnsupported) || songs != nil {
		t.Fatalf("qianqian GetHotSongs error mismatch: songs=%v err=%v", songs, err)
	}
}
