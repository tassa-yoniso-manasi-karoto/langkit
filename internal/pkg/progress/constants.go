package progress

// Canonical bar ID prefixes - use these as prefixes, suffixes allowed for uniqueness.
// The importance allocation system uses prefix matching, so bar IDs like
// "demucs-process-12345" will match BarDemucsProcess.
const (
	// BarMediaBar tracks total video files processed (bulk mode only)
	BarMediaBar = "media-bar"

	// BarItemBar tracks subtitle lines processed (subs2cards, subs2dubs, condense)
	BarItemBar = "item-bar"

	// Demucs (voice enhancement) bars
	BarDemucsProcess  = "demucs-process"   // Voice separation processing
	BarDemucsDockerDL = "demucs-docker-dl" // Docker image download (~7GB)
	BarDemucsModelDL  = "demucs-model-dl"  // Model weights download

	// Audio-separator / MelBand RoFormer bars
	BarAudioSepProcess  = "audiosep-process"   // Voice separation processing
	BarAudioSepDockerDL = "audiosep-docker-dl" // Docker image download
	BarAudioSepModelDL  = "audiosep-model-dl"  // Model weights download

	// Transliteration bars
	BarTranslitProcess  = "translit-process"   // Romanization/tokenization processing
	BarTranslitDockerDL = "translit-docker-dl" // Provider Docker image download
	BarTranslitInit     = "translit-init"      // Database initialization (e.g., Ichiran)

	// Expectation checker bars
	BarCheckProbe  = "check-probe"  // Per-file probe (mediainfo) progress
	BarCheckDecode = "check-decode" // Per-file decode integrity progress
)
