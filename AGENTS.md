# Agent Instructions

**CRITICAL: Always update this file when adding new features, changing architecture, or modifying build/deploy processes.**

This is a fork of [miniflux/v2](https://github.com/miniflux/v2) maintained at [theblazehen/miniflux](https://github.com/theblazehen/miniflux).

## Fork-Specific Changes

### EntryReadingTime Filter Rule
- **Location**: `internal/reader/filter/filter.go`
- **What**: New `EntryReadingTime` rule type for Entry Block/Allow Rules
- **Syntax**: `EntryReadingTime=>2`, `EntryReadingTime=<5`, `EntryReadingTime=>=10`, `EntryReadingTime=!=0`
- **Operators**: `>`, `<`, `>=`, `<=`, `=`, `!=` — value is in minutes
- **Tests**: `internal/reader/filter/filter_test.go`

### On-Demand Discussion Links (HN, Reddit)
- **Location**: `internal/discussion/` (new package)
- **What**: On-demand lookup of Hacker News and Reddit discussion threads for entry URLs
- **API endpoint**: `GET /v1/entries/{entryID}/discussions` — returns `{"discussions": [...]}`
- **UI endpoint**: `GET /entry/{entryID}/discussions` — same response, used by JS
- **Providers**: HN (Algolia API), Reddit (search.json). Lobsters code exists but is disabled (requires auth).
- **Caching**: In-memory TTL cache (30min results, 15min empty, 5min error). Singleflight dedup.
- **UI rendering**: Entry detail page, below date/reading time. JS in `app.js` (`initializeDiscussionLinks()`).
- **Template**: `internal/template/templates/views/entry.html` — `div.entry-discussions`
- **Wiring**: `Finder` singleton created in `internal/http/server/routes.go`, injected into both API and UI handlers.
- **Handler files**: `internal/api/discussion_handlers.go`, `internal/ui/entry_discussions.go`

## Build & Deploy

### Docker / GHCR
- **Workflow**: `.github/workflows/ghcr.yml`
- **Registry**: `ghcr.io/theblazehen/miniflux`
- **Triggers**: Push to `main`, semver tags
- **Platforms**: `linux/amd64`, `linux/arm64`
- **Dockerfile**: `packaging/docker/alpine/Dockerfile`

### Local build
```bash
make miniflux          # build binary
go test ./...          # run tests
go build ./...         # verify compilation
```

## Architecture Notes

- Go project, no frameworks. Templates use Go `html/template` with `embed`.
- JS is vanilla, in `internal/ui/static/js/app.js`. No build step.
- All static files embedded via `//go:embed`.
- API handlers: `internal/api/` — REST API with auth middleware.
- UI handlers: `internal/ui/` — server-rendered HTML + JSON endpoints for JS.
- Both share `*storage.Storage` and `*worker.Pool` via handler structs.
- Discussion `*discussion.Finder` also shared via handler structs.

## Conventions

- Follow existing Miniflux code style exactly (slog for logging, no panics in handlers, etc.)
- Tests in `_test.go` files in the same package
- SPDX license headers on all Go files
- Import path comments: `package foo // import "miniflux.app/v2/internal/foo"`
