package ai

import (
    "context"
    "io"
    "os/exec"
)

// ExternalFFmpegTranscoder uses the `ffmpeg` CLI to transcode audio to PCM16 LE
// 16kHz mono. It spawns ffmpeg and returns a reader for the stdout stream.
type ExternalFFmpegTranscoder struct{
    // Path can be set to override ffmpeg binary location; empty uses PATH lookup.
    Path string
}

func (t *ExternalFFmpegTranscoder) Transcode(r io.Reader) (io.Reader, error) {
    bin := t.Path
    if bin == "" {
        bin = "ffmpeg"
    }
    // Build ffmpeg command: read from stdin, output raw PCM signed 16-bit little-endian
    // at 16kHz mono to stdout.
    cmd := exec.CommandContext(context.Background(), bin,
        "-hide_banner", "-loglevel", "error",
        "-i", "pipe:0",
        "-f", "s16le",
        "-ar", "16000",
        "-ac", "1",
        "pipe:1",
    )

    stdin, err := cmd.StdinPipe()
    if err != nil {
        return nil, err
    }
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return nil, err
    }

    if err := cmd.Start(); err != nil {
        // if ffmpeg cannot be started, return original reader as-is with error
        _ = stdin.Close()
        return r, err
    }

    // copy from provided reader into ffmpeg stdin until EOF, then close stdin
    go func() {
        defer stdin.Close()
        io.Copy(stdin, r)
    }()

    return stdout, nil
}
