package ai

import (
    "io"
)

// PCMTranscoder is a placeholder for a real Opus->PCM16 transcoder.
// For now it acts as a pass-through and returns the original reader.
// Replace with a real implementation when adding native or cgo-based
// decoding (e.g., using libopus) or an external process.
type PCMTranscoder struct{}

func (t *PCMTranscoder) Transcode(r io.Reader) (io.Reader, error) {
    // no-op for now
    return r, nil
}
