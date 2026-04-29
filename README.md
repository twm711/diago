# AI Call Center Platform Workspace

This workspace contains the new call-center platform in `platform/`.
The platform backend depends on `github.com/emiago/diago` as a Go module, and this checkout uses a local replace target for development convenience.

## Active workspace

- `platform/`: Go backend, MySQL compose file, and React + Ant Design frontend
- `kernel/diago`: local upstream mirror for the diago dependency during development

## Platform layout

- `platform/cmd/api`: Go API server entrypoint
- `platform/internal/config`: environment configuration
- `platform/internal/db`: MySQL and GORM wiring
- `platform/internal/call`: call/session domain model and service
- `platform/internal/webrtcgw`: Pion WebRTC gateway scaffold
- `platform/internal/ai`: ASR/TTS/LLM gateway abstraction
- `platform/internal/httpapi`: HTTP routes and handlers
- `platform/web`: React + Ant Design console
- `platform/docker-compose.yml`: local MySQL development stack

## Local run

```bash
cd platform
go mod tidy
go test ./... -run '^$'
go run ./cmd/api
```

Frontend:

```bash
cd platform/web
npm install
npm run dev
```

Database:

```bash
cd platform
docker compose up -d mysql
```
