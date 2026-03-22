# Vault

A read-only file server with a Blender-inspired dark UI. Browse directories, preview files, and share direct download URLs — all from a single Docker container.

## Features

- **Blender-style UI** — dark theme with beveled widgets, disclosure panels, and the Inter typeface
- **File browsing** — navigate directories with breadcrumbs, back/forward history, and keyboard support
- **Previews** — images (with checkerboard alpha), PDFs, text/code, video, and audio
- **Direct file URLs** — every file is accessible at `/files/path/to/file.ext`, usable in `<img>` tags, `curl`, etc.
- **List and grid views** — toggle between a detailed list and a thumbnail grid
- **Sidebar (N-panel)** — collapsible panels showing directory info and selected file details
- **Filter** — instantly filter the current directory by filename
- **Persistent settings** — view mode, sidebar state, and last path are saved in cookies
- **Read-only** — no upload, delete, or modify endpoints
- **Zero Go dependencies** — backend uses only the standard library
- **Minimal frontend deps** — Svelte 5, Vite, and the Svelte Vite plugin

## Quick Start

### Docker (recommended)

```bash
# Clone and place your files in ./contents/
docker compose up --build

# Visit http://localhost:8080
```

The `contents/` directory is mounted read-only into the container. Any files you place there will be browsable through the web UI.

### Local Development

Requires Go 1.22+ and Node.js 18+.

```bash
# Terminal 1: backend (serves API on :8080)
cd backend
CONTENT_ROOT=../contents go run .

# Terminal 2: frontend (dev server on :5173, proxies API to :8080)
cd frontend
npm install
npm run dev
```

Or use the Makefile:

```bash
make dev-backend   # in one terminal
make dev-frontend  # in another
```

### Production Build (without Docker)

```bash
make build
# Binary at ./vault, frontend at ./frontend/dist/

CONTENT_ROOT=/path/to/your/files ./vault
```

## Configuration

| Environment Variable | Default      | Description                           |
|---------------------|--------------|---------------------------------------|
| `CONTENT_ROOT`      | `/contents`  | Path to the directory to serve        |
| `PORT`              | `8080`       | Port the server listens on            |

## API

### `GET /api/browse?path=/`

Returns a JSON listing of the directory at the given path.

**Response:**

```json
{
  "path": "/",
  "entries": [
    {
      "name": "photos",
      "path": "/photos",
      "isDir": true,
      "size": 0,
      "modTime": "2026-03-22T10:00:00Z",
      "ext": ""
    },
    {
      "name": "readme.txt",
      "path": "/readme.txt",
      "isDir": false,
      "size": 1234,
      "modTime": "2026-03-22T10:00:00Z",
      "ext": ".txt"
    }
  ]
}
```

Entries are sorted: directories first, then files alphabetically. Hidden files (starting with `.`) are excluded.

**Errors:**

| Status | Reason              |
|--------|---------------------|
| 400    | Invalid or traversal path |
| 400    | Path is a file, not a directory |
| 404    | Path does not exist  |
| 500    | Cannot read directory |

### `GET /files/{path}`

Serves the raw file at the given path. Suitable for embedding:

```html
<img src="https://your-vault.example.com/files/photos/cat.jpg" />
```

```bash
curl -O https://your-vault.example.com/files/documents/report.pdf
```

## Project Structure

```
vault/
├── backend/
│   ├── go.mod
│   ├── main.go           # HTTP server, browse API, file serving
│   └── main_test.go      # Unit tests (14 tests, ~64% coverage)
├── frontend/
│   ├── package.json       # Svelte 5 + Vite
│   ├── vite.config.js
│   ├── index.html
│   └── src/
│       ├── main.js
│       ├── App.svelte     # Root layout, global Blender styles
│       ├── FileBrowser.svelte  # File listing, toolbar, sidebar
│       └── Preview.svelte # File preview panel
├── docker-compose.yml
├── Dockerfile             # Multi-stage: Node → Go → Alpine
├── Makefile
├── LICENSE
└── contents/              # Your files go here (git-ignored)
```

## Tests

```bash
cd backend
go test -v -cover ./...
```

Tests cover:
- Directory listing and metadata
- Hidden file exclusion
- Sort order (directories first, then alphabetical)
- Subdirectory navigation
- Path traversal prevention
- Non-existent and invalid paths
- Direct file serving
- JSON content-type headers

## Security

- **Read-only** — the server only serves files, no write operations
- **Path traversal protection** — all paths are cleaned and verified to stay within `CONTENT_ROOT`
- **Hidden files excluded** — dotfiles are never listed in the browse API
- **Docker volume is read-only** — `docker-compose.yml` mounts contents with `:ro`

## License

MIT
