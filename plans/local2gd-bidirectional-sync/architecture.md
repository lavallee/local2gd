# Architecture Design: local2gd — Bidirectional Local Folder ↔ Google Drive Sync

**Date:** 2026-03-25
**Mindset:** MVP
**Scale:** Personal/Team CLI tool
**Status:** Approved

---

## Technical Summary

local2gd is a single Go binary organized into five internal packages: `cmd` (CLI entry points), `auth` (OAuth2 flow and token storage), `gdrive` (Google Drive/Docs API client), `convert` (bidirectional markdown↔Docs conversion), and `sync` (the core sync engine that orchestrates change detection, conflict resolution, and state management).

Data flows through a pipeline: **scan** both sides → **diff** against stored base state → **classify** each file (new/changed/deleted/conflict) → **act** (convert + push/pull) → **update** state. The conversion layer is the only complex piece — everything else is plumbing.

Local state lives in a `.local2gd/` directory within each synced folder, containing the base versions of all files (for future three-way merge) and a `state.json` mapping local paths to Drive file IDs and content hashes. Global config lives in XDG config dir.

## System Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    CLI (cobra)                           │
│  auth  ·  sync [--dry-run]  ·  status  ·  diff          │
└────────────┬────────────────────────────────────────────┘
             │
┌────────────▼────────────────────────────────────────────┐
│                  Sync Engine                             │
│  scan local  ·  scan remote  ·  diff vs base            │
│  classify actions  ·  execute plan  ·  update state     │
└──────┬──────────────────┬───────────────────────────────┘
       │                  │
┌──────▼──────┐   ┌──────▼──────┐
│  Converter  │   │   GDrive    │
│  md → docs  │   │   Client    │
│  docs → md  │   │  Drive API  │
│  (goldmark) │   │  Docs API   │
└─────────────┘   └──────┬──────┘
                         │
                  ┌──────▼──────┐
                  │    Auth     │
                  │   OAuth2    │
                  │ token store │
                  └─────────────┘
```

## Technology Stack

| Layer | Choice | Rationale |
|-------|--------|-----------|
| Language | Go 1.23+ | Single binary distribution, strong Google API libs, precedent (rclone, gh) |
| CLI | cobra + viper | De facto Go CLI standard, XDG-aware config via viper |
| Markdown | goldmark + extensions | CommonMark compliant, extensible AST, GFM support |
| Google APIs | `google.golang.org/api/drive/v3`, `docs/v1` | Official, auto-generated, typed structs |
| OAuth2 | `golang.org/x/oauth2/google` | Standard Go OAuth2 library |
| Config | TOML via viper | Human-editable, cleaner than YAML for simple config |
| XDG paths | `adrg/xdg` | Clean XDG Base Directory spec implementation |
| Logging | `log/slog` (stdlib) | Structured logging, zero deps |
| Testing | stdlib `testing` + `testify` | Assertions + table-driven tests |
| Distribution | goreleaser | Cross-compile + brew tap + GitHub releases |

## Components

### `cmd/` — CLI Layer
- **Purpose:** Parse commands, flags, and config. Delegate to sync engine.
- **Responsibilities:**
  - `auth` — trigger OAuth flow, print success/failure
  - `sync` — load config, instantiate engine, run sync, print results
  - `status` — run scan + diff only, print change summary (P1)
  - `--dry-run` — pass flag to engine, engine prints plan without acting
- **Dependencies:** cobra, viper, sync engine
- **Interface:** User runs `local2gd <command> [flags]`

### `internal/auth/` — OAuth2 & Token Management
- **Purpose:** Handle Google OAuth2 authorization code flow, store/load refresh tokens.
- **Responsibilities:**
  - Start local HTTP server on random port for OAuth callback
  - Open browser to Google consent screen
  - Exchange authorization code for access + refresh tokens
  - Store encrypted refresh token in `$XDG_DATA_HOME/local2gd/token.json`
  - Load and refresh access token on demand
  - Provide `*http.Client` to other packages (authorized client)
- **Dependencies:** `golang.org/x/oauth2/google`, `adrg/xdg`
- **Interface:** `auth.Login() error`, `auth.Client() (*http.Client, error)`

### `internal/gdrive/` — Google Drive & Docs Client
- **Purpose:** Thin wrapper over Google Drive v3 and Docs v1 APIs. Isolates API details from sync logic.
- **Responsibilities:**
  - List files in a Drive folder (recursive, with metadata)
  - Export a Google Doc as markdown (`files.export` with `text/markdown`)
  - Create a Google Doc from structured content (`docs.documents.create` + `batchUpdate`)
  - Update an existing Google Doc (`batchUpdate` with full replacement)
  - Upload/download binary files
  - Delete files (move to Drive trash)
  - Resolve Drive folder path to folder ID
  - Handle pagination, rate limiting (exponential backoff), and retries
- **Dependencies:** `google.golang.org/api/drive/v3`, `docs/v1`, auth package
- **Interface:**
  - `gdrive.NewClient(httpClient) *Client`
  - `client.ListFolder(folderID) ([]FileInfo, error)`
  - `client.ExportMarkdown(fileID) ([]byte, error)`
  - `client.CreateDoc(folderID, title, batchRequests) (FileInfo, error)`
  - `client.UpdateDoc(fileID, batchRequests) error`
  - `client.ResolvePath(path) (folderID, error)`

### `internal/convert/` — Bidirectional Markdown ↔ Google Docs Conversion
- **Purpose:** Convert between local markdown and Google Docs API structures.
- **Responsibilities:**
  - **MD → Docs:** Parse markdown with goldmark into AST → walk AST → emit `batchUpdate` requests (InsertText, UpdateTextStyle, UpdateParagraphStyle, InsertInlineImage, InsertTable)
  - **Docs → MD:** Post-process the `text/markdown` export from Google (fix code blocks, clean up image references, normalize whitespace)
  - Track character offsets for `batchUpdate` index positioning
  - Handle P0 elements: headings (H1-H6), bold, italic, bold+italic, links, ordered lists, unordered lists, horizontal rules, paragraphs
  - Graceful degradation: unsupported elements become plain text with a warning, not an error
- **Dependencies:** goldmark, `google.golang.org/api/docs/v1` types
- **Interface:**
  - `convert.MarkdownToDocs(mdContent []byte) ([]*docs.Request, string, error)` — returns batchUpdate requests and extracted title
  - `convert.PostProcessExport(rawMD []byte) []byte` — clean up Google's markdown export

### `internal/sync/` — Sync Engine
- **Purpose:** Orchestrate the full sync operation: scan, diff, plan, execute, update state.
- **Responsibilities:**
  - Scan local directory tree for `.md` files
  - Scan remote Drive folder via gdrive client
  - Load state from `.local2gd/state.json`
  - Compute content hashes (SHA-256) for local files
  - Classify each file into an action: `push`, `pull`, `create-local`, `create-remote`, `delete-local`, `delete-remote`, `conflict`, `unchanged`
  - In dry-run mode: print the plan and stop
  - Execute the plan: call converter + gdrive client for each action
  - On conflict: prompt user via stdin (pick local / pick remote / skip)
  - Update state.json and base copies after each successful file sync
  - Report results (files synced, conflicts, errors)
- **Dependencies:** convert, gdrive, auth
- **Interface:**
  - `sync.NewEngine(client *gdrive.Client, config PairingConfig) *Engine`
  - `engine.Run(dryRun bool) (*Report, error)`

## Data Model

### Global Config (`~/.config/local2gd/config.toml`)

```toml
[pairings.notes]
local = "~/Documents/notes"
remote = "Notes"  # path relative to My Drive root

[pairings.work]
local = "~/Documents/work-docs"
remote = "Engineering/Docs"
```

### Per-Pairing State (`.local2gd/state.json` in local folder)

```json
{
  "version": 1,
  "remote_folder_id": "1abc...",
  "last_sync": "2026-03-25T10:30:00Z",
  "files": {
    "design.md": {
      "drive_file_id": "1xyz...",
      "local_hash": "sha256:aaa...",
      "remote_hash": "sha256:bbb...",
      "last_synced": "2026-03-25T10:30:00Z"
    }
  }
}
```

### Base Copies (`.local2gd/base/`)

Mirrors local directory structure. Each file is a copy of its content at last successful sync, used as common ancestor for P1 three-way merge.

### Relationships

- Config pairing → references one local dir + one Drive path
- State file → lives inside the local dir of a pairing
- State entry → maps local filename to Drive file ID + hashes
- Base copy → mirrors the local file at last sync point

## APIs / Interfaces

### Google Drive API v3
- **Type:** REST (via Go client library)
- **Purpose:** File operations — list, export, create, delete
- **Key Methods:**
  - `files.list` — list files in a folder with query filter
  - `files.export` — export Google Doc as `text/markdown`
  - `files.create` — create new file/folder
  - `files.delete` — move to trash

### Google Docs API v1
- **Type:** REST (via Go client library)
- **Purpose:** Structured document manipulation
- **Key Methods:**
  - `documents.create` — create blank document
  - `documents.batchUpdate` — apply structured changes (insert text, apply styles)

### Internal Package Interfaces

- `auth.Client() (*http.Client, error)` — get authorized HTTP client
- `gdrive.NewClient(httpClient) *Client` — create Drive/Docs client
- `convert.MarkdownToDocs([]byte) ([]*docs.Request, string, error)` — convert markdown to Docs API requests
- `convert.PostProcessExport([]byte) []byte` — clean up exported markdown
- `sync.NewEngine(*gdrive.Client, PairingConfig) *Engine` — create sync engine
- `engine.Run(dryRun bool) (*Report, error)` — execute sync

## Implementation Phases

### Phase 1: Skeleton + Auth
**Goal:** Bootable CLI that can authenticate with Google.
- Initialize Go module, cobra CLI scaffold with `main.go` and `cmd/` package
- Implement OAuth2 browser flow with local callback server
- Store/load refresh token in XDG data dir with 0600 permissions
- `local2gd auth` works end-to-end
- **Verification:** Run `local2gd auth`, complete browser flow, token stored, re-run without re-auth.

### Phase 2: Google Drive Client
**Goal:** Read and write to Google Drive.
- Implement `gdrive.Client`: list folder, resolve path, export markdown, create doc (empty), delete
- Wire authorized HTTP client from auth package
- Handle pagination and rate limiting with exponential backoff
- **Verification:** List contents of a real Drive folder, export a Google Doc as markdown, create a new empty Doc.

### Phase 3: Markdown → Docs Conversion
**Goal:** Convert local markdown files into real Google Docs.
- Implement goldmark AST walker that emits `batchUpdate` requests
- P0 elements: paragraphs, headings, bold, italic, links, ordered/unordered lists
- Index tracking strategy: build content bottom-up or track cumulative offsets
- Integrate with `gdrive.CreateDoc` and `gdrive.UpdateDoc`
- Unit tests for each element type with round-trip assertions
- **Verification:** Convert a representative markdown file, create Google Doc, manually verify formatting.

### Phase 4: Docs → Markdown Export
**Goal:** Convert Google Docs back to clean markdown.
- Use `gdrive.ExportMarkdown` (Drive API `text/markdown`)
- Implement `convert.PostProcessExport` — normalize whitespace, fix known issues
- Evaluate export quality against representative documents
- **Verification:** Export a Google Doc, compare to expected markdown. Round-trip test: MD→Doc→MD, diff output.

### Phase 5: Sync Engine (MVP)
**Goal:** Full sync loop works end-to-end.
- Implement local scanner (walk directory, compute SHA-256 hashes)
- Implement remote scanner (list Drive folder, export + hash for change detection)
- State management: load/save `state.json`, read/write base copies
- Change classification logic: compare local hash, remote hash, and stored hashes
- Action execution: push, pull, create-local, create-remote, conflict prompt
- `--dry-run` flag: print plan without executing
- User-facing output: progress messages, sync summary report
- **Verification:** Set up test pairing with real Drive folder. Create files on both sides, run sync, verify propagation. Modify on both sides, verify changes sync. Modify same file on both sides, verify conflict prompt.

### Phase 6: Polish & Distribution
**Goal:** Installable, usable tool.
- Helpful error messages (auth expired, network failure, invalid config)
- goreleaser config for cross-platform builds + brew tap + GitHub releases
- README with usage examples and known limitations
- `.gitignore` template recommending `.local2gd/` exclusion
- **Verification:** `brew install` on clean machine, full auth + sync flow succeeds.

## Technical Risks

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| `text/markdown` export quality insufficient for round-trips | H | M | Phase 4 evaluates early. Fallback: HTML export + custom converter. Conversion layer is modular. |
| `batchUpdate` index tracking is error-prone (character offsets shift on insertion) | M | H | Build content bottom-up (reverse order) to avoid offset shifting. Thorough unit tests per element type. |
| Google API rate limits on large initial sync | M | M | Sequential file processing with built-in backoff. Progress reporting so user knows it's working. |
| OAuth token storage security on shared machines | L | L | Store in XDG data dir with 0600 permissions. Document that tokens grant Drive access. |
| Goldmark AST doesn't expose enough structure for Docs mapping | M | L | Goldmark is extensible. Can add custom AST node handlers. Worst case: supplement with regex post-processing. |

## Dependencies

### External
- **Google Drive API v3** — file listing, export, upload, deletion
- **Google Docs API v1** — document creation and batchUpdate for structured content
- **Google OAuth2** — authorization code flow for user consent
- **git** (optional, P1 only) — `git merge-file` for three-way merge

### Internal
- N/A (greenfield project, no existing code to integrate with)

## Integration Impact

> Greenfield project — no existing consumers or deprecated code.

### New Components → Existing Consumers

| New Component | Existing Consumer | Integration Action |
|--------------|-------------------|-------------------|
| `cmd/` | None (entry point) | N/A |
| `internal/auth` | `cmd/auth.go`, `cmd/sync.go` | CLI commands call `auth.Client()` |
| `internal/gdrive` | `internal/sync` | Sync engine uses gdrive client |
| `internal/convert` | `internal/sync` | Sync engine calls converter for each file |
| `internal/sync` | `cmd/sync.go` | CLI instantiates and runs engine |

### Deprecated/Replaced Code

N/A — greenfield project.

### Integration Verification

- **Phase 1:** `local2gd auth` completes browser flow, token persists across runs
- **Phase 2:** `local2gd auth` → authorized client → list real Drive folder contents
- **Phase 3:** Local `.md` file → Google Doc created with correct formatting
- **Phase 4:** Google Doc → exported markdown matches expectations
- **Phase 5:** Full bidirectional sync loop with real Drive folder: create, modify, conflict

## Security Considerations

- OAuth refresh tokens stored with `0600` file permissions in XDG data dir
- No secrets in config file — config only contains folder paths
- OAuth client ID/secret embedded in binary (standard for installed apps per Google's guidance)
- `.local2gd/` directory should be added to `.gitignore` (contains state and base copies)
- No sensitive data logged — file names logged at info level, content never logged

## Future Considerations

- **P1: Three-way merge** — base copies stored from Phase 5. Adding `git merge-file` is a targeted change in `sync/actions.go`.
- **P1: Multiple pairings** — config structure already supports it. Sync engine takes a single pairing config; CLI loops over pairings.
- **P1: Deletion with soft-delete** — add `.local2gd/trash/` directory, move instead of delete, add cleanup on age.
- **P1: Frontmatter preservation** — strip YAML frontmatter before Docs upload, re-attach on export. Store in state or base copy.
- **P2: Images** — extract image URLs from Docs API JSON, download to assets dir, rewrite markdown references.
- **P2: Tables** — goldmark GFM extension parses tables. Add table→Docs and Docs→table in `convert/`.
- **v2: Git-as-infrastructure** — replace `.local2gd/base/` with hidden git repo for full version history and rollback.

---

**Next Step:** Run `cub itemize` to generate implementation tasks.
