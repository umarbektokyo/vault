# Vault

A file server with a Blender-inspired dark UI. Browse directories, preview files, upload from the browser, and protect files with secret codes — all from a single Docker container.

**[GitHub](https://github.com/umarbektokyo/vault)**

## Features

- **Blender-style UI** — dark theme with beveled widgets, disclosure panels, and the Inter typeface
- **File browsing** — navigate directories with breadcrumbs, back/forward history, and keyboard support
- **Previews** — images (with checkerboard alpha), PDFs, text/code, video, and audio
- **Direct file URLs** — every file is accessible at `/files/path/to/file.ext`, usable in `<img>` tags, `curl`, etc.
- **List and grid views** — toggle between a detailed list and a thumbnail grid
- **Sidebar (N-panel)** — collapsible panels showing directory info and selected file details
- **Filter** — instantly filter the current directory by filename
- **Persistent settings** — view mode, sidebar state, and last path are saved in cookies
- **Authentication** — optional single-user login via environment variables, enabling uploads and file management
- **File upload** — authenticated users can upload files directly from the browser
- **Secret codes** — protect individual files with a secret code; files remain visible but require the code to download or preview
- **Zero Go dependencies** — backend uses only the standard library
- **Minimal frontend deps** — Svelte 5, Vite, and the Svelte Vite plugin

## Quick Start

### Docker (recommended)

```bash
# Clone and place your files in ./contents/
docker compose up --build

# Visit http://localhost:8080
```

The `contents/` directory is mounted into the container. Any files you place there will be browsable through the web UI. By default, authentication is enabled with `admin`/`changeme` — change these in `docker-compose.yml`.

### Local Development

Requires Go 1.22+ and Node.js 18+.

```bash
# Terminal 1: backend (serves API on :8080)
cd backend
CONTENT_ROOT=../contents VAULT_USER=admin VAULT_PASS=secret go run .

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
| `VAULT_USER`        | *(unset)*    | Login username (auth disabled if unset) |
| `VAULT_PASS`        | *(unset)*    | Login password (auth disabled if unset) |
| `VAULT_SECRET`      | *(derived)*  | HMAC signing key for cookies (auto-derived from `VAULT_PASS` if not set) |

When both `VAULT_USER` and `VAULT_PASS` are set, the UI shows a Login button. Authenticated users can upload files and manage secret codes. Without these variables, the server runs in public read-only mode.

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
      "ext": "",
      "hasSecret": false
    },
    {
      "name": "readme.txt",
      "path": "/readme.txt",
      "isDir": false,
      "size": 1234,
      "modTime": "2026-03-22T10:00:00Z",
      "ext": ".txt",
      "hasSecret": true
    }
  ]
}
```

Entries are sorted: directories first, then files alphabetically. Hidden files (starting with `.`) are excluded. The `hasSecret` field indicates whether a file is protected with a secret code.

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

Files protected with a secret code return `403` unless the request includes a valid unlock cookie or an authenticated session cookie.

### `GET /api/auth`

Returns the current authentication status.

```json
{ "authEnabled": true, "authenticated": false }
```

### `POST /api/login`

Authenticates with username and password. Sets a signed session cookie (7-day expiry).

```json
{ "username": "admin", "password": "secret" }
```

### `POST /api/logout`

Clears the session cookie.

### `POST /api/upload` *(requires auth)*

Uploads a file via multipart form. Fields: `file` (the file) and `path` (target directory, defaults to `/`). Max 100 MB.

### `POST /api/secret` *(requires auth)*

Sets or removes a secret code on a file.

```json
{ "path": "/readme.txt", "code": "my-secret" }
```

Send an empty `code` to remove the secret:

```json
{ "path": "/readme.txt", "code": "" }
```

### `POST /api/unlock`

Validates a secret code and sets a signed unlock cookie (24-hour expiry) granting access to the file.

```json
{ "path": "/readme.txt", "code": "my-secret" }
```

## Project Structure

```
vault/
├── backend/
│   ├── go.mod
│   ├── main.go           # HTTP server, auth, upload, secrets, file serving
│   └── main_test.go      # 28 unit tests
├── frontend/
│   ├── package.json       # Svelte 5 + Vite
│   ├── vite.config.js
│   ├── index.html         # SEO meta tags, OG, Twitter Card
│   ├── public/
│   │   ├── favicon.svg    # Padlock icon (Blender-style branding)
│   │   ├── og.svg         # Social media preview source
│   │   ├── og.png         # Social media preview (1200x630)
│   │   └── robots.txt
│   └── src/
│       ├── main.js
│       ├── App.svelte     # Root layout, global Blender styles
│       ├── FileBrowser.svelte  # File listing, toolbar, sidebar, modals
│       └── Preview.svelte # File preview panel
├── docker-compose.yml
├── Dockerfile             # Multi-stage: Node → Go → Alpine
├── Makefile
├── STYLE_GUIDE.md         # Branding guide (favicon, OG image, SEO)
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
- Authentication (login, logout, session validation)
- Upload (auth required, file creation)
- Secret codes (set, remove, verify, `hasSecret` in browse)
- Unlock flow (correct code, wrong code, cookie-based access)
- Auth bypass for secret-coded files

## Security

- **Path traversal protection** — all paths are cleaned and verified to stay within `CONTENT_ROOT`
- **Hidden files excluded** — dotfiles are never listed in the browse API
- **HMAC-signed cookies** — session and unlock cookies are signed with HMAC-SHA256 to prevent tampering
- **Secret codes hashed** — stored as HMAC-SHA256 hashes, never in plaintext
- **Auth-gated mutations** — upload and secret management endpoints require authentication
- **Upload sanitization** — filenames are sanitized; dotfiles and traversal names are rejected

## License

MIT
