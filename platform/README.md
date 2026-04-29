# Platform Layer

This is the active AI call-center application.
It uses Go for backend services, MySQL for persistence, React + Ant Design for the UI, and Pion WebRTC for browser media access.

## What exists now

- `cmd/api`: Go HTTP API server
- `internal/db`: MySQL + GORM bootstrap
- `internal/call`: session and tenant domain models
- `internal/webrtcgw`: Pion WebRTC gateway scaffold
- `internal/ai`: ASR/TTS/LLM adapter abstraction
- `web`: React + Ant Design dashboard
- `docker-compose.yml`: local MySQL stack

## Diago integration

The backend module depends on `github.com/emiago/diago` through `platform/go.mod`.
For local development this checkout uses a replace target so the dependency resolves from the workspace.

## Run order

1. Start MySQL with `docker compose up -d mysql`.
2. Run the API server with `go run ./cmd/api`.
3. Start the frontend with `npm run dev` inside `web/`.

## Next implementation steps

- Persist WebRTC sessions and bridge them to diago call sessions.
- Add ASR/TTS provider adapters and streaming voice pipeline.
- Add tenant, auth, call-routing, and agent-console APIs.
