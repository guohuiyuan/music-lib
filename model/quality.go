package model

import "strings"

// AudioQuality is a source-independent quality identifier. Sources may map the
// same identifier to different formats and bitrates.
type AudioQuality string

const (
	AudioQualityAuto     AudioQuality = "auto"
	AudioQualityPreview  AudioQuality = "preview"
	AudioQualityLow      AudioQuality = "low"
	AudioQualityStandard AudioQuality = "standard"
	AudioQualityHigher   AudioQuality = "higher"
	AudioQualityHigh     AudioQuality = "high"
	AudioQualityLossless AudioQuality = "lossless"
	AudioQualityHiRes    AudioQuality = "hires"
	AudioQualityUltra    AudioQuality = "ultra"
	AudioQualitySpatial  AudioQuality = "spatial"
	AudioQualityVivid    AudioQuality = "vivid"
	AudioQualityAtmos    AudioQuality = "atmos"
	AudioQualityAtmos51  AudioQuality = "atmos_51"
	AudioQualityMaster   AudioQuality = "master"
)

// AudioQualityOption describes one selectable quality tier.
type AudioQualityOption struct {
	Value   AudioQuality `json:"value"`
	Label   string       `json:"label"`
	Format  string       `json:"format"`
	Bitrate int          `json:"bitrate,omitempty"`
	VIP     bool         `json:"vip,omitempty"`
}

// NormalizeAudioQuality accepts canonical values and common bitrate aliases.
func NormalizeAudioQuality(value string) (AudioQuality, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "auto", "":
		return AudioQualityAuto, true
	case "preview", "sample", "trial", "try":
		return AudioQualityPreview, true
	case "low", "64", "64k", "lq":
		return AudioQualityLow, true
	case "standard", "128", "128k", "m500", "pq":
		return AudioQualityStandard, true
	case "higher", "192", "192k", "medium":
		return AudioQualityHigher, true
	case "high", "320", "320k", "m800", "hq", "exhigh", "highest":
		return AudioQualityHigh, true
	case "lossless", "flac", "f000":
		return AudioQualityLossless, true
	case "hires", "hi_res", "hi-res", "24bit", "sq", "zq", "zq24":
		return AudioQualityHiRes, true
	case "ultra", "640", "640k", "ogg", "o801":
		return AudioQualityUltra, true
	case "spatial", "sky", "jyeffect":
		return AudioQualitySpatial, true
	case "vivid":
		return AudioQualityVivid, true
	case "atmos", "q000", "dolby":
		return AudioQualityAtmos, true
	case "atmos_51", "spatial_51", "5.1", "q001":
		return AudioQualityAtmos51, true
	case "master", "ai00", "zq32", "jymaster":
		return AudioQualityMaster, true
	default:
		return "", false
	}
}
