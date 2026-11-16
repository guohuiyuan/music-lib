package netease

import (
	"fmt"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
)

const neteaseQualityProbeSongID = "496869422"
const neteaseQualityProbeKeyword = "周杰伦"

type neteaseQualityCandidate struct {
	option model.AudioQualityOption
	levels []string
}

var neteaseQualityCandidates = []neteaseQualityCandidate{
	{option: model.AudioQualityOption{Value: model.AudioQualityMaster, Label: "超清母带", Format: "wav", VIP: true}, levels: []string{"jymaster"}},
	{option: model.AudioQualityOption{Value: model.AudioQualitySpatial, Label: "沉浸环绕声", Format: "flac", VIP: true}, levels: []string{"sky", "jyeffect"}},
	{option: model.AudioQualityOption{Value: model.AudioQualityVivid, Label: "臻音全景声", Format: "mp3", VIP: true}, levels: []string{"vivid"}},
	{option: model.AudioQualityOption{Value: model.AudioQualityAtmos, Label: "杜比全景声", Format: "flac", VIP: true}, levels: []string{"dolby"}},
	{option: model.AudioQualityOption{Value: model.AudioQualityHiRes, Label: "Hi-Res 无损", Format: "flac", VIP: true}, levels: []string{"hires"}},
	{option: model.AudioQualityOption{Value: model.AudioQualityLossless, Label: "无损音质", Format: "flac", VIP: true}, levels: []string{"lossless"}},
	{option: model.AudioQualityOption{Value: model.AudioQualityHigh, Label: "高品质 320k", Format: "mp3", Bitrate: 320}, levels: []string{"exhigh"}},
	{option: model.AudioQualityOption{Value: model.AudioQualityHigher, Label: "较高品质 192k", Format: "mp3", Bitrate: 192}, levels: []string{"higher"}},
	{option: model.AudioQualityOption{Value: model.AudioQualityStandard, Label: "标准音质 128k", Format: "mp3", Bitrate: 128}, levels: []string{"standard"}},
}

// ListQualityOptions returns all quality tiers understood by NetEase.
func ListQualityOptions() []model.AudioQualityOption {
	options := make([]model.AudioQualityOption, 0, len(neteaseQualityCandidates))
	for _, candidate := range neteaseQualityCandidates {
		options = append(options, candidate.option)
	}
	return options
}

// AvailableQualityOptions probes the current account against NetEase's player
// endpoint. Actual entitlement is validated by the returned stream, not by a
// standalone VIP flag.
func (n *Netease) AvailableQualityOptions() ([]model.AudioQualityOption, error) {
	if n.availableQualityCache != nil {
		return append([]model.AudioQualityOption(nil), n.availableQualityCache...), nil
	}

	fixedProbeSong := &model.Song{
		Source: "netease",
		ID:     neteaseQualityProbeSongID,
	}
	options, firstErr := n.probeNeteaseQualityLadder([]*model.Song{fixedProbeSong})
	if len(options) == 0 {
		probeSongs := make([]*model.Song, 0, 5)
		probeSongs = append(probeSongs, fixedProbeSong)
		seenSongIDs := map[string]struct{}{neteaseQualityProbeSongID: {}}
		if songs, err := n.Search(neteaseQualityProbeKeyword); err == nil {
			for i := range songs {
				if i >= 4 {
					break
				}
				if _, exists := seenSongIDs[songs[i].ID]; exists {
					continue
				}
				if songs[i].Source == "" {
					songs[i].Source = "netease"
				}
				seenSongIDs[songs[i].ID] = struct{}{}
				probeSongs = append(probeSongs, &songs[i])
			}
		}
		options, searchErr := n.probeNeteaseQualityLadder(probeSongs)
		if firstErr == nil {
			firstErr = searchErr
		}
		if len(options) > 0 {
			n.availableQualityCache = append([]model.AudioQualityOption(nil), options...)
			return append([]model.AudioQualityOption(nil), options...), nil
		}
	}
	if len(options) > 0 {
		n.availableQualityCache = append([]model.AudioQualityOption(nil), options...)
		return append([]model.AudioQualityOption(nil), options...), nil
	}
	if firstErr != nil {
		return nil, firstErr
	}
	n.availableQualityCache = []model.AudioQualityOption{}
	return []model.AudioQualityOption{}, nil
}

func (n *Netease) probeNeteaseQualityLadder(
	probeSongs []*model.Song,
) ([]model.AudioQualityOption, error) {
	var firstErr error
	for candidateIndex, candidate := range neteaseQualityCandidates {
		for _, song := range probeSongs {
			_, err := n.GetDownloadURLWithQuality(song, string(candidate.option.Value))
			if err == nil {
				return neteaseQualityLadderFrom(candidateIndex), nil
			}
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return nil, firstErr
}

// neteaseQualityLadderFrom returns the successfully probed tier and every
// lower tier. Account entitlement is a capability range: a 320 kbps stream
// also implies 192 kbps and 128 kbps availability even if an individual
// track's lower-tier endpoint temporarily rejects a probe.
func neteaseQualityLadderFrom(highestAvailableIndex int) []model.AudioQualityOption {
	if highestAvailableIndex < 0 || highestAvailableIndex >= len(neteaseQualityCandidates) {
		return nil
	}
	options := make([]model.AudioQualityOption, 0, len(neteaseQualityCandidates)-highestAvailableIndex)
	for _, candidate := range neteaseQualityCandidates[highestAvailableIndex:] {
		options = append(options, candidate.option)
	}
	return options
}

func neteaseCandidateForQuality(quality model.AudioQuality) (neteaseQualityCandidate, bool) {
	switch quality {
	case model.AudioQualityMaster:
		return neteaseQualityCandidates[0], true
	case model.AudioQualitySpatial:
		return neteaseQualityCandidates[1], true
	case model.AudioQualityVivid:
		return neteaseQualityCandidates[2], true
	case model.AudioQualityAtmos:
		return neteaseQualityCandidates[3], true
	case model.AudioQualityStandard:
		return neteaseQualityCandidates[8], true
	case model.AudioQualityHigher:
		return neteaseQualityCandidates[7], true
	case model.AudioQualityHigh:
		return neteaseQualityCandidates[6], true
	case model.AudioQualityLossless:
		return neteaseQualityCandidates[5], true
	case model.AudioQualityHiRes:
		return neteaseQualityCandidates[4], true
	default:
		return neteaseQualityCandidate{}, false
	}
}

func neteaseLevelsForQuality(quality string) ([]string, bool) {
	normalized, ok := model.NormalizeAudioQuality(quality)
	if !ok {
		return nil, false
	}
	candidate, ok := neteaseCandidateForQuality(normalized)
	if !ok {
		return nil, false
	}
	return append([]string(nil), candidate.levels...), true
}

func neteaseLevelForQuality(quality string) (string, bool) {
	levels, ok := neteaseLevelsForQuality(quality)
	if !ok || len(levels) == 0 {
		return "", false
	}
	return levels[0], true
}

func neteaseQualityForLevel(level string) (model.AudioQuality, bool) {
	level = strings.ToLower(strings.TrimSpace(level))
	for _, candidate := range neteaseQualityCandidates {
		for _, candidateLevel := range candidate.levels {
			if level == candidateLevel {
				return candidate.option.Value, true
			}
		}
	}
	return "", false
}

func neteaseAudioQuality(bitrate int, ext string, level string) model.AudioQuality {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "jymaster":
		return model.AudioQualityMaster
	case "sky", "jyeffect":
		return model.AudioQualitySpatial
	case "vivid":
		return model.AudioQualityVivid
	case "dolby":
		return model.AudioQualityAtmos
	case "hires":
		return model.AudioQualityHiRes
	case "lossless":
		return model.AudioQualityLossless
	case "exhigh":
		return model.AudioQualityHigh
	case "higher":
		return model.AudioQualityHigher
	case "standard":
		return model.AudioQualityStandard
	}

	ext = strings.ToLower(strings.TrimSpace(ext))
	switch {
	case ext == "wav" && bitrate >= 2_000_000:
		return model.AudioQualityMaster
	case (ext == "flac" || ext == "wav") && bitrate >= 1_000_000:
		return model.AudioQualityHiRes
	case ext == "flac" || ext == "wav" || bitrate >= 700_000:
		return model.AudioQualityLossless
	case bitrate >= 280_000:
		return model.AudioQualityHigh
	case bitrate >= 160_000:
		return model.AudioQualityHigher
	case bitrate >= 96_000:
		return model.AudioQualityStandard
	default:
		return model.AudioQualityLow
	}
}

func neteaseQualityAtLeast(actual model.AudioQuality, requested model.AudioQuality) bool {
	rank := func(quality model.AudioQuality) int {
		switch quality {
		case model.AudioQualityMaster:
			return 9
		case model.AudioQualitySpatial:
			return 8
		case model.AudioQualityVivid:
			return 7
		case model.AudioQualityAtmos:
			return 6
		case model.AudioQualityHiRes:
			return 5
		case model.AudioQualityLossless:
			return 4
		case model.AudioQualityHigh:
			return 3
		case model.AudioQualityHigher:
			return 2
		case model.AudioQualityStandard:
			return 1
		default:
			return 0
		}
	}
	return rank(actual) >= rank(requested) && rank(requested) > 0
}

func neteaseQualityError(quality string) error {
	return fmt.Errorf("netease audio quality %s is unavailable for this song or account", quality)
}
