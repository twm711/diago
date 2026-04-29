package ai

import (
    "context"
    "io"
)

// Provider is an abstraction for ASR providers. Implementations should consume audio
// from the provided Reader and return a channel of ASRResult. Implementations are
// responsible for decoding/transcoding audio as needed by the provider.
type Provider interface {
    Start(ctx context.Context, pcID string, r io.Reader) (<-chan ASRResult, error)
    Stop(pcID string)
}

// Transcoder is a pluggable audio transcoder. For real providers implement
// conversion (e.g., Opus -> PCM16). For now this is a placeholder identity
// transcoder used by the stub provider.
type Transcoder interface {
    Transcode(r io.Reader) (io.Reader, error)
}

// IdentityTranscoder returns the reader as-is.
type IdentityTranscoder struct{}

func (i *IdentityTranscoder) Transcode(r io.Reader) (io.Reader, error) { return r, nil }
