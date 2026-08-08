package profile

// Preset is the single ordered quality catalog shared by runtime transcode,
// ABR playlists and the legacy preprocess pipeline. Runtime and persistent ABR
// bitrates remain explicit policies because they optimize different workloads;
// resolution, naming and audio policy must not drift between pipelines.
type Preset struct {
	Name                   string
	Width                  int
	Height                 int
	AudioBitrate           string
	RuntimeVideoBitrate    string
	PersistentVideoBitrate string
	PersistentMaxBitrate   string
	PersistentBufSize      string
}

// EncodingProfile is the transport-neutral shape consumed by FFmpeg adapters
// and exposed by the legacy ABR status API.
type EncodingProfile struct {
	Name         string `json:"name"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	VideoBitrate string `json:"video_bitrate"`
	AudioBitrate string `json:"audio_bitrate"`
	MaxBitrate   string `json:"max_bitrate"`
	BufSize      string `json:"buf_size"`
}

var catalog = []Preset{
	{Name: "360p", Width: 640, Height: 360, AudioBitrate: "96k", RuntimeVideoBitrate: "800k", PersistentVideoBitrate: "800k", PersistentMaxBitrate: "1200k", PersistentBufSize: "1600k"},
	{Name: "480p", Width: 854, Height: 480, AudioBitrate: "128k", RuntimeVideoBitrate: "1500k", PersistentVideoBitrate: "1400k", PersistentMaxBitrate: "2100k", PersistentBufSize: "2800k"},
	{Name: "720p", Width: 1280, Height: 720, AudioBitrate: "128k", RuntimeVideoBitrate: "3000k", PersistentVideoBitrate: "2800k", PersistentMaxBitrate: "4200k", PersistentBufSize: "5600k"},
	{Name: "1080p", Width: 1920, Height: 1080, AudioBitrate: "192k", RuntimeVideoBitrate: "6000k", PersistentVideoBitrate: "5000k", PersistentMaxBitrate: "7500k", PersistentBufSize: "10000k"},
	{Name: "2K", Width: 2560, Height: 1440, AudioBitrate: "192k", RuntimeVideoBitrate: "12000k", PersistentVideoBitrate: "10000k", PersistentMaxBitrate: "15000k", PersistentBufSize: "20000k"},
	{Name: "4K", Width: 3840, Height: 2160, AudioBitrate: "256k", RuntimeVideoBitrate: "25000k", PersistentVideoBitrate: "20000k", PersistentMaxBitrate: "30000k", PersistentBufSize: "40000k"},
}

// Presets returns a copy so callers cannot mutate the process-wide catalog.
func Presets() []Preset {
	result := make([]Preset, len(catalog))
	copy(result, catalog)
	return result
}

func Find(name string) (Preset, bool) {
	for _, preset := range catalog {
		if preset.Name == name {
			return preset, true
		}
	}
	return Preset{}, false
}

func Runtime(name string) (EncodingProfile, bool) {
	preset, ok := Find(name)
	if !ok {
		return EncodingProfile{}, false
	}
	return EncodingProfile{
		Name:         preset.Name,
		Width:        preset.Width,
		Height:       preset.Height,
		VideoBitrate: preset.RuntimeVideoBitrate,
		AudioBitrate: preset.AudioBitrate,
	}, true
}

func Persistent(name string) (EncodingProfile, bool) {
	preset, ok := Find(name)
	if !ok {
		return EncodingProfile{}, false
	}
	return persistentProfile(preset), true
}

func PersistentProfiles() []EncodingProfile {
	profiles := make([]EncodingProfile, 0, len(catalog))
	for _, preset := range catalog {
		profiles = append(profiles, persistentProfile(preset))
	}
	return profiles
}

func Names() []string {
	names := make([]string, 0, len(catalog))
	for _, preset := range catalog {
		names = append(names, preset.Name)
	}
	return names
}

func NamesUpToHeight(height int) []string {
	names := make([]string, 0, len(catalog))
	for _, preset := range catalog {
		if preset.Height <= height {
			names = append(names, preset.Name)
		}
	}
	return names
}

func HighestPersistentAtOrBelow(height int) (EncodingProfile, bool) {
	var selected Preset
	found := false
	for _, preset := range catalog {
		if preset.Height <= height && (!found || preset.Height > selected.Height) {
			selected = preset
			found = true
		}
	}
	if !found {
		return EncodingProfile{}, false
	}
	return persistentProfile(selected), true
}

func persistentProfile(preset Preset) EncodingProfile {
	return EncodingProfile{
		Name:         preset.Name,
		Width:        preset.Width,
		Height:       preset.Height,
		VideoBitrate: preset.PersistentVideoBitrate,
		AudioBitrate: preset.AudioBitrate,
		MaxBitrate:   preset.PersistentMaxBitrate,
		BufSize:      preset.PersistentBufSize,
	}
}
