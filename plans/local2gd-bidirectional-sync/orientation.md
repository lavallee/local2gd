# Orient Report: local2gd — Bidirectional Local Folder ↔ Google Drive Sync

**Date:** 2026-03-25
**Orient Depth:** Standard
**Status:** Approved

---

## Executive Summary

A Go CLI tool that bidirectionally syncs local markdown files with Google Drive, converting to/from native Google Docs transparently. Designed for power users who write in markdown locally and need their work accessible as real Google Docs for collaborators. Manual sync with conflict detection and user-prompted resolution.

## Problem Statement

Power users who work in local markdown (Obsidian, vim, VS Code, etc.) cannot share their work as native Google Docs without manual copy-paste per file. Google Drive for Desktop treats markdown as opaque files — no conversion happens. This creates a two-world problem: the author's workflow is markdown, their collaborators' workflow is Google Docs, and bridging them is entirely manual.

## Refined Vision

A command-line tool (`local2gd`) that maintains a bidirectional mapping between a local directory of markdown files and a Google Drive folder of native Google Docs. Running `local2gd sync` detects changes on both sides, converts between formats, and reconciles differences — allowing the user to edit markdown locally while collaborators interact with real Google Docs.

## Requirements

### P0 - Must Have

- **`local2gd auth`** — OAuth browser flow, store refresh token in XDG data dir. Minimal friction first-run.
- **`local2gd sync`** — Sync a single configured local↔Drive pairing. The core operation.
- **Markdown → Google Doc creation** — Convert headings, bold/italic, links, ordered/unordered lists into native Google Docs via the Docs API `batchUpdate`. Created Docs must be editable by collaborators.
- **Google Doc → Markdown export** — Use Google Drive API `text/markdown` export as baseline. Post-process known issues where feasible.
- **State tracking** — Per-file: Drive file ID, content hash (SHA-256), last sync timestamp. Stored in `.local2gd/state.json` within the local folder.
- **Change detection** — Compare current content hash against stored base to determine what changed on each side.
- **File-level conflict resolution** — When both sides changed, prompt user to pick local or remote version. No merging in P0.
- **New file propagation** — New files on either side are created on the other side.
- **XDG-compliant config** — `~/.config/local2gd/config.toml` with at least one pairing definition.
- **`--dry-run` flag** — Preview what sync would do without making changes. Essential safety net.

### P1 - Should Have

- **Multiple pairings** — Configure several local↔Drive folder pairs, sync individually (`local2gd sync notes`) or all (`local2gd sync`).
- **Three-way merge** — Use `git merge-file` with stored base version to auto-merge non-overlapping changes. Fall back to "pick one" on conflicts.
- **Deletion propagation with soft-delete** — Deletions propagate across sides, but deleted files go to `.local2gd/trash/` with 30-day retention before permanent removal.
- **`local2gd status`** — Show what's changed on each side since last sync without syncing.
- **Sidecar metadata** — Preserve Google Doc comments, permissions, and properties in `.local2gd/meta/<filename>.json`.
- **Google Doc title from heading** — Set Doc title from first `# heading` in markdown, fall back to filename.
- **Frontmatter preservation** — YAML frontmatter survives round-trips (stripped before Docs upload, re-attached on export).
- **`local2gd diff [path]`** — Show content diff of what would change on sync.

### P2 - Nice to Have

- **Image handling** — Extract images from Docs to local assets directory, upload local images to Drive and reference in Docs.
- **Table support** — Markdown tables ↔ Google Docs tables.
- **Code block fidelity** — Post-process native markdown export to fix code block formatting.
- **`local2gd init`** — Interactive wizard to set up a new pairing.
- **Shared Drive support** — Sync to Google Shared Drives (different permission model).
- **Binary file pass-through** — Non-markdown files sync as-is without conversion.
- **Sync individual files** — `local2gd sync path/to/file.md` for targeted sync.

## Constraints

- **Obsidian compatibility** — Markdown must remain valid for Obsidian and similar tools. No proprietary extensions. Stick to CommonMark + GFM (tables, strikethrough, task lists). Frontmatter is fine (Obsidian uses it). Wikilinks (`[[...]]`) are not generated but should be preserved if present.
- **Git runtime dependency** — `git merge-file` is required for P1 three-way merge. Acceptable for power-user audience. P0 works without git.
- **OAuth requires browser** — First-run auth needs a desktop environment. No headless/CI support in v1.
- **Google API rate limits** — ~300 requests/100 seconds for Drive API. Large syncs need batching and backoff.
- **One-to-one file mapping** — Each local markdown file maps to exactly one Google Doc. No splitting or combining.

## Assumptions

- User has a Google account with Drive access
- User has a browser available for OAuth flow
- Target audience is developers/power users comfortable with CLI tools
- Google's `text/markdown` export MIME type remains available (relatively new feature)
- Google Shared Drives behave similarly to My Drive for basic file operations (deferred to P2 for explicit support)
- Local filesystem is case-sensitive or user avoids case-only-different filenames

## Open Questions / Experiments

- **Google `text/markdown` export quality** — How well does it handle our target document types (prose-heavy with headings, lists, links)? → Experiment: prototype export of 10 representative Docs and evaluate output quality.
- **First sync bootstrap** — When pointing at an existing Drive folder and an existing local folder, how should initial state be established? → Experiment: try three approaches (import-only, export-only, interactive file-by-file) and see which feels right.
- **Google Drive filename collisions** — Drive allows duplicate names in one folder; local FS doesn't. → Experiment: detect and suffix with Drive file ID on conflict. Evaluate UX.
- **Sidecar format** — JSON proposed, but evaluate whether hidden dotfiles (`.design-notes.md.json`) or a single directory (`.local2gd/meta/`) is less disruptive to Obsidian and other tools. → Decide during architecture.

## Out of Scope

- Sheets↔CSV or Slides↔Markdown conversion
- Continuous/watch-based or scheduled sync
- Multi-editor real-time collaboration scenarios
- Service account / headless CI auth
- CRDT-based merge or real-time conflict resolution
- Version history / rollback (consider git-as-infrastructure in v2)
- Google Docs comments or suggestions rendered in markdown
- Web UI or GUI of any kind

## Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Google removes/changes `text/markdown` export | H | Fallback path: HTML export + custom converter. Modular conversion layer. |
| Round-trip degradation accumulates over many syncs | H | Hash-based skip-if-unchanged prevents unnecessary re-conversion. Store base version. |
| Accidental deletion propagation destroys data | H | Soft-delete buffer in `.local2gd/trash/` with 30-day retention (P1). `--dry-run` for P0. |
| OAuth token expiry mid-sync on large trees | M | Refresh token proactively before sync. Handle 401 mid-operation with retry. |
| Google API rate limiting on large initial sync | M | Batch API calls, exponential backoff, progress reporting, `--dry-run` to preview scope. |
| Markdown AST → Docs batchUpdate is complex and brittle | M | Start with minimal element set (P0). Expand incrementally. Extensive round-trip tests. |

## MVP Definition

The smallest useful version:

1. **`local2gd auth`** — Open browser, authorize, store refresh token.
2. **`local2gd sync`** — For a single configured pairing:
   - Scan local folder and Drive folder
   - Detect new, changed, and deleted files on each side (via content hash against stored state)
   - New local `.md` → create Google Doc (headings, bold/italic, links, lists)
   - New Google Doc → export as local `.md`
   - Changed on one side only → sync the change
   - Changed on both sides → prompt user to pick local or remote
   - `--dry-run` to preview without acting
3. **Config** — Single TOML file with one pairing: `local = "~/path"`, `remote = "Drive/path"`.

This gives a usable sync loop for the primary use case (author writes markdown, collaborators read/edit Google Docs) within a focused build scope.

---

**Next Step:** Run `cub architect` to proceed to technical design.
