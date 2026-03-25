---
status: researching
priority: high
complexity: high
dependencies: []
blocks: []
created: 2026-03-25
updated: 2026-03-25
readiness:
  score: 6
  blockers: []
  questions:
    - How well does Google's native text/markdown export handle our specific document types? (needs hands-on testing)
    - What sidecar metadata format best balances completeness with not cluttering the workspace?
  decisions_needed:
    - Sidecar format design (JSON? YAML? hidden dotfiles?)
    - Whether to use gws CLI as transport or call Google APIs directly from Go
  tools_needed:
    - Google Drive API (v3)
    - Google Docs API (v1)
    - goldmark (Go markdown parser)
    - cobra + viper (Go CLI framework)
    - goreleaser (distribution)
---

# local2gd — Bidirectional Local Folder ↔ Google Drive Sync

## Overview

A CLI tool that bidirectionally syncs local directories with Google Drive folders, converting Google Docs to/from Markdown transparently. Designed for power users who work in markdown locally but need files accessible as native Google Docs for collaborators. Manual sync with auto-merge and user-prompted conflict resolution.

## Goals

- Bidirectional sync between local markdown files and Google Docs, with Docs stored as native Google Docs (not uploaded .md files)
- 1:1 directory structure mapping between local and remote folders
- Support multiple local/remote folder pairings via XDG-compliant configuration
- Sync at the pairing level or individual file level (`local2gd sync` or `local2gd sync path/to/file.md`)
- Sidecar metadata files to preserve Google Doc properties (comments, permissions, version history) across round-trips
- Auto-merge non-overlapping changes using three-way merge; prompt user to "pick one" on conflicts
- Non-Google-Doc files (images, PDFs, etc.) sync as binary pass-through
- Markdown extensions supported but compatible with tools like Obsidian (no clobbering)
- Clear communication of which Google Docs features survive markdown round-trips and which don't

## Non-Goals

- Sheets↔CSV, Slides↔Markdown, or other format translations (v1)
- Continuous/watch-based or scheduled sync (manual only for v1)
- High-volume multi-editor collaborative scenarios
- Service account / headless auth (OAuth browser flow only for v1)
- CRDTs or real-time collaborative editing infrastructure
- Version history / rollback (consider git-as-infrastructure for v2)

## Design / Approach

### Language & Stack

**Go** — chosen for single-binary distribution (`brew install`, GitHub releases), strong ecosystem precedent (rclone, gws, gh CLI), and solid Google API libraries.

| Concern | Library |
|---------|---------|
| CLI framework | `cobra` + `viper` |
| Google APIs | `google.golang.org/api/docs/v1`, `google.golang.org/api/drive/v3` |
| OAuth2 | `golang.org/x/oauth2/google` |
| Markdown parsing | `goldmark` (with extensions for tables, strikethrough, task lists) |
| XDG config | `adrg/xdg` |
| Distribution | `goreleaser` |
| Logging | `log/slog` (stdlib) |
| Three-way merge | Shell out to `git merge-file` |

### Markdown ↔ Google Docs Conversion (Hybrid Approach)

**Google Docs → Markdown:**
- Use Google Drive API's native `text/markdown` export (`files.export` with MIME type `text/markdown`) as baseline
- Post-process to fix known issues (code blocks not supported in native export, images exported as base64 blobs)
- Fall back to Docs API JSON → custom converter for elements the native export handles poorly

**Markdown → Google Docs:**
- Parse markdown into AST using goldmark
- Generate Google Docs API `batchUpdate` requests from the AST
- Map: headings → named styles, bold/italic → text styles, links → link objects, lists → list/bullet objects, tables → table structures, images → inline image objects (uploaded to Drive first)

**Prior art to study:** The Joplin Google Docs sync plugin uses a three-tier architecture (Markdown → intermediate representation → Docs API) and is the most complete bidirectional implementation found. Worth examining for design patterns, even though the code is Joplin-specific.

**Known round-trip limitations (to document for users):**
- Google Docs comments and suggestions are preserved in sidecar but not represented in markdown
- Embedded drawings have no markdown equivalent
- Complex table formatting (merged cells, colored cells) degrades to simple markdown tables
- Code blocks may lose language hints on Docs→MD via native export (post-processing can help)

### Sync Architecture (Unison-style + diff3)

**State tracking:** Hidden directory `.local2gd/` in each synced local folder:
- `base/` — copy of every file as it was at last successful sync (the common ancestor for three-way merge)
- `state.json` — metadata: file paths, content hashes, Google Drive file IDs, last sync timestamps, revision IDs

**Sync algorithm:**
1. **Scan** both sides (local filesystem, Google Drive API listing)
2. **Compare** each file against the base version using content hashes:
   - Changed only locally → convert and push to Google Drive, update base
   - Changed only remotely → export and pull to local, update base
   - Changed on neither → no action
   - Deleted on one side, unchanged on other → propagate deletion
   - New file on one side → convert and copy to the other side
   - **Changed on both sides** → attempt three-way merge
3. **Three-way merge** (for files changed on both sides):
   - Run `git merge-file` with base, local, and remote versions
   - Clean merge → apply to both sides, update base
   - Conflicts → present user with: accept local, accept remote, or show diff

### Configuration (XDG-compliant)

Config at `~/.config/local2gd/config.toml` (or `$XDG_CONFIG_HOME/local2gd/config.toml`):

```toml
[pairings.notes]
local = "~/Documents/notes"
remote = "My Drive/Notes"  # Google Drive path

[pairings.work]
local = "~/Documents/work-docs"
remote = "Shared drives/Engineering/Docs"

[settings]
sidecar_dir = ".local2gd"  # hidden directory in each local folder
conflict_style = "pick-one"  # v1 only supports pick-one
```

Auth tokens stored in `~/.local/share/local2gd/` (or `$XDG_DATA_HOME/local2gd/`).

### Sidecar Metadata

For each synced file `document.md`, a sidecar at `.local2gd/meta/document.md.json`:

```json
{
  "drive_file_id": "1abc...",
  "drive_revision_id": "r123",
  "last_synced": "2026-03-25T10:30:00Z",
  "content_hash": "sha256:...",
  "google_metadata": {
    "comments": [...],
    "permissions": [...],
    "properties": {...}
  }
}
```

## Implementation Notes

### CLI Commands (Planned)

```
local2gd init                      # Initialize a pairing (interactive)
local2gd sync [path]               # Sync all pairings, or a specific file/pairing
local2gd status                    # Show sync status (what's changed on each side)
local2gd auth                      # OAuth browser flow, store refresh token
local2gd config                    # View/edit configuration
local2gd diff [path]               # Show what would change on sync
```

### Google Workspace CLI (`gws`) Assessment

The `gws` CLI (github.com/googleworkspace/cli, Rust, 22k stars) is very new (23 days old, pre-1.0) with breaking changes expected. It handles auth and API calls well but does not address the hard problems (format conversion, conflict resolution). **Decision: evaluate as optional transport layer but do not take a hard dependency. Call Google APIs directly from Go for v1.**

### Key Technical Risks

1. **Conversion fidelity** — the hybrid approach (native export + custom import) creates an asymmetric pipeline. Docs→MD and MD→Docs may not be perfectly inverse. Mitigation: extensive round-trip testing, clear documentation of limitations.
2. **Google API rate limits** — large folder syncs may hit Drive API quotas. Mitigation: batch requests, exponential backoff, progress reporting.
3. **Image handling** — images in Google Docs need to be extracted and stored locally (likely in a sibling assets directory). Images in local markdown need to be uploaded to Drive before referencing in Docs API calls.

## Open Questions

1. How well does Google's native `text/markdown` export handle our specific document types? (needs hands-on prototyping)
2. What sidecar metadata format best balances completeness with not cluttering the workspace? (design during architecture, current proposal above is a starting point)

## Future Considerations

- **Continuous sync** via filesystem watcher + Drive Changes API (v2)
- **Git-as-infrastructure** for version history and rollback (v2)
- **Sheets↔CSV** and other format translations (v2+)
- **Service account auth** for headless/CI use (v2)
- **CRDT-based merge** if real-time multi-editor scenarios become relevant (v3+)
- **Conflict resolution upgrade** from "pick one" to git-style conflict markers or interactive merge (v2)
- **Obsidian/PKM plugin** that wraps this CLI for tighter integration

## Research Findings

### Conversion Approaches Evaluated

| Approach | Docs→MD | MD→Docs | Round-trip Fidelity | Effort |
|----------|---------|---------|---------------------|--------|
| Google Drive API `text/markdown` export | Good for prose, poor for code/images | Import exists but less documented | Low-Medium | Minimal |
| Pandoc via DOCX intermediate | Medium (lossy hop) | Medium (lossy hop) | Medium | Minimal |
| Custom via Docs API JSON | Full control | Full control | Highest potential | High |
| **Hybrid (chosen)** | Native export + post-processing | Custom AST→batchUpdate | Medium-High | Medium |

### Merge Approaches Evaluated

| Approach | Complexity | Merge Quality | Dependencies | Recommendation |
|----------|-----------|---------------|-------------|----------------|
| CRDT (Yjs/Automerge) | High | High (overkill) | Heavy, poor Go support | Skip for v1 |
| diff3 / three-way merge | Low | High for text | `git merge-file` (already installed) | **Chosen for v1** |
| Git as infrastructure | Medium | High + history | git | Consider for v2 |
| File-level pick-one (rclone-style) | Minimal | None (no merging) | None | Too basic for text files |

### Prior Art

- **Joplin Google Docs plugin** — most complete bidirectional implementation. Three-tier pipeline architecture worth studying.
- **rclone bisync** — good model for state tracking (listing files from last sync). File-level only, no content merge.
- **Unison** — closest architectural precedent for the sync algorithm. Base-version comparison, prompt on conflicts.
- **md2docs** (Python) — early stage, not production ready. Individual file conversion only.
- **docs-markdown** (Node.js) — Docs API JSON to markdown converter, useful reference for field mapping.

---

**Status**: researching
**Last Updated**: 2026-03-25
