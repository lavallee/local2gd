# Itemized Plan: local2gd — Bidirectional Local Folder ↔ Google Drive Sync

> Source: [local2gd-bidirectional-sync.md](../../specs/researching/local2gd-bidirectional-sync.md)
> Orient: [orientation.md](./orientation.md) | Architect: [architecture.md](./architecture.md)
> Generated: 2026-03-25

## Context Summary

A Go CLI tool that bidirectionally syncs local markdown files with Google Drive, converting Google Docs to/from markdown transparently. Power users edit markdown locally; collaborators interact with native Google Docs.

**Mindset:** MVP | **Scale:** Personal/Team CLI

---

## Epic: local2gd-a0 - local2gd-bidirectional-sync #1: Skeleton + Auth

Priority: 0
Labels: phase-1, setup

Initialize the Go project structure and implement Google OAuth2 authentication. This is the foundation everything else builds on.

### Task: local2gd-a0.1 - Initialize Go module and cobra CLI scaffold

Priority: 0
Labels: phase-1, setup, model:haiku, complexity:low

**Context**: Create the Go module, directory structure, and cobra root command. This establishes the project skeleton that all other work builds into.

**Implementation Steps**:
1. Run `go mod init github.com/lavallee/local2gd`
2. Create directory structure: `cmd/`, `internal/auth/`, `internal/gdrive/`, `internal/convert/`, `internal/sync/`
3. Create `main.go` that calls `cmd.Execute()`
4. Create `cmd/root.go` with cobra root command (`local2gd` with version flag and description)
5. Add placeholder subcommands: `cmd/auth.go`, `cmd/sync.go`
6. Add `adrg/xdg` and `cobra`/`viper` dependencies

**Acceptance Criteria**:
- [ ] `go build` produces a `local2gd` binary
- [ ] `local2gd --help` shows usage with `auth` and `sync` subcommands
- [ ] `local2gd --version` prints version
- [ ] Directory structure matches architecture spec

**Files**: main.go, cmd/root.go, cmd/auth.go, cmd/sync.go, go.mod

---

### Task: local2gd-a0.2 - Implement OAuth2 browser flow

Priority: 0
Labels: phase-1, setup, model:sonnet, complexity:medium
Blocks: local2gd-a1.1

**Context**: Users need to authorize local2gd to access their Google Drive. This implements the standard OAuth2 authorization code flow with a local callback server.

**Implementation Steps**:
1. Create `internal/auth/oauth.go` — define OAuth2 config with Drive + Docs scopes
2. Implement `Login()`: start local HTTP server on random port, open browser to Google consent URL, handle callback to receive authorization code, exchange for tokens
3. Use `golang.org/x/oauth2/google` for the OAuth2 flow
4. Embed OAuth client ID/secret as constants (standard for installed apps per Google guidance)
5. Create a Google Cloud project with Drive API + Docs API enabled, generate OAuth2 client ID for desktop app

**Acceptance Criteria**:
- [ ] `Login()` opens browser and completes OAuth flow
- [ ] Access token and refresh token are returned
- [ ] Handles user cancellation gracefully (timeout after 2 minutes)

**Files**: internal/auth/oauth.go

---

### Task: local2gd-a0.3 - Implement token storage and refresh

Priority: 0
Labels: phase-1, setup, model:sonnet, complexity:medium
Blocks: local2gd-a1.1

**Context**: Tokens must persist across CLI invocations so users don't re-auth every time. Refresh tokens must be stored securely.

**Implementation Steps**:
1. Create `internal/auth/token.go` — token file path at `$XDG_DATA_HOME/local2gd/token.json`
2. Implement `SaveToken(token *oauth2.Token) error` — write JSON with 0600 permissions, create parent dir if needed
3. Implement `LoadToken() (*oauth2.Token, error)` — read and deserialize
4. Implement `Client() (*http.Client, error)` — load token, create OAuth2 client that auto-refreshes, save refreshed token on use
5. Handle missing token (return clear error: "run `local2gd auth` first")

**Acceptance Criteria**:
- [ ] Token file created at correct XDG path with 0600 permissions
- [ ] Subsequent calls to `Client()` reuse stored token without re-auth
- [ ] Token auto-refreshes when expired and new token is persisted
- [ ] Clear error message when no token exists

**Files**: internal/auth/token.go

---

### Task: local2gd-a0.4 - Wire auth into CLI command

Priority: 0
Labels: phase-1, setup, model:haiku, complexity:low

**Context**: Connect the auth package to the `local2gd auth` cobra command so users can authenticate end-to-end.

**Implementation Steps**:
1. Update `cmd/auth.go` to call `auth.Login()` then `auth.SaveToken()`
2. Print success message with authenticated email (from token info endpoint)
3. If already authenticated, print status and ask to re-auth with `--force` flag

**Acceptance Criteria**:
- [ ] `local2gd auth` completes full browser OAuth flow and stores token
- [ ] Re-running `local2gd auth` reports already authenticated
- [ ] `local2gd auth --force` re-authenticates
- [ ] `local2gd sync` (without auth) prints "run `local2gd auth` first"

**Files**: cmd/auth.go

---

> **Checkpoint: Auth works end-to-end.** User can run `local2gd auth` and get an authorized session that persists.

---

## Epic: local2gd-a1 - local2gd-bidirectional-sync #2: Google Drive Client

Priority: 0
Labels: phase-2, api

Implement the Google Drive and Docs API client layer. This provides all the remote operations the sync engine needs.

### Task: local2gd-a1.1 - Implement Drive file listing and folder resolution

Priority: 0
Labels: phase-2, api, model:sonnet, complexity:medium

**Context**: The sync engine needs to list files in a Drive folder and resolve human-readable paths (like "Notes/Projects") to Drive folder IDs.

**Implementation Steps**:
1. Create `internal/gdrive/client.go` — `NewClient(httpClient *http.Client) *Client` constructor, initialize Drive and Docs services
2. Create `internal/gdrive/files.go` — define `FileInfo` struct (ID, Name, MimeType, ModifiedTime, MD5Checksum)
3. Implement `ListFolder(folderID string) ([]FileInfo, error)` — use `files.list` with query `'{folderID}' in parents`, handle pagination
4. Implement `ResolvePath(path string) (string, error)` — split path on `/`, resolve each segment by listing parent and matching name, return final folder ID. Handle "My Drive" root as `"root"`.
5. Add exponential backoff wrapper for all API calls (start at 1s, max 30s, max 5 retries)

**Acceptance Criteria**:
- [ ] `ListFolder` returns all files in a Drive folder with correct metadata
- [ ] `ListFolder` handles pagination for folders with >100 files
- [ ] `ResolvePath("Notes/Projects")` returns the correct folder ID
- [ ] `ResolvePath` with nonexistent path returns clear error
- [ ] API rate limit errors trigger retry with backoff

**Files**: internal/gdrive/client.go, internal/gdrive/files.go

---

### Task: local2gd-a1.2 - Implement markdown export and doc creation

Priority: 0
Labels: phase-2, api, model:sonnet, complexity:medium

**Context**: The two core API operations for sync: exporting a Google Doc as markdown, and creating a new Google Doc with structured content.

**Implementation Steps**:
1. Create `internal/gdrive/docs.go`
2. Implement `ExportMarkdown(fileID string) ([]byte, error)` — use `files.export` with MIME type `text/markdown`, return raw bytes
3. Implement `CreateDoc(folderID, title string, requests []*docs.Request) (FileInfo, error)` — create blank doc in folder via Drive API (set parents), then apply `batchUpdate` with provided requests, return file info
4. Implement `UpdateDoc(fileID string, requests []*docs.Request) error` — clear existing content (delete all), then apply `batchUpdate` with new requests
5. Implement `DeleteFile(fileID string) error` — move to Drive trash via `files.update` with `trashed: true`

**Acceptance Criteria**:
- [ ] `ExportMarkdown` returns markdown content for a real Google Doc
- [ ] `CreateDoc` creates a Google Doc in the specified folder with correct title
- [ ] `UpdateDoc` replaces document content
- [ ] `DeleteFile` moves file to Drive trash (not permanent delete)
- [ ] All operations are wired through the backoff wrapper

**Files**: internal/gdrive/docs.go

---

> **Checkpoint: Drive client works.** Can list folders, export docs, create docs, delete files via the API.

---

## Epic: local2gd-a2 - local2gd-bidirectional-sync #3: Markdown → Docs Conversion

Priority: 0
Labels: phase-3, logic, risk:high

Convert local markdown into Google Docs API batchUpdate requests. This is the most technically challenging component.

### Task: local2gd-a2.1 - Set up goldmark parser with GFM extensions

Priority: 0
Labels: phase-3, logic, model:sonnet, complexity:low

**Context**: Before building the converter, we need a properly configured markdown parser that handles the elements we support.

**Implementation Steps**:
1. Create `internal/convert/md_to_docs.go` — import goldmark with GFM extension (tables, strikethrough, task lists)
2. Create a parser function that takes `[]byte` markdown and returns a goldmark AST
3. Create helper to extract title from first H1 node (for Google Doc title), falling back to empty string
4. Write table-driven tests with sample markdown inputs verifying AST structure

**Acceptance Criteria**:
- [ ] Parser handles: headings, paragraphs, bold, italic, links, ordered lists, unordered lists, horizontal rules
- [ ] Title extraction returns first H1 content or empty string
- [ ] Tests cover each element type

**Files**: internal/convert/md_to_docs.go, internal/convert/md_to_docs_test.go

---

### Task: local2gd-a2.2 - Implement AST walker that emits batchUpdate requests

Priority: 0
Labels: phase-3, logic, model:opus, complexity:high

**Context**: The core conversion: walk the goldmark AST and produce Google Docs API `batchUpdate` requests. The key challenge is managing character offset indices — all Docs API insertions use absolute character positions.

**Implementation Steps**:
1. Create `internal/convert/elements.go` — define helper functions for building Docs API request types: `insertText(index, text)`, `updateTextStyle(startIndex, endIndex, bold, italic, link)`, `updateParagraphStyle(startIndex, endIndex, namedStyle)`
2. Implement `MarkdownToDocs(mdContent []byte) ([]*docs.Request, string, error)`:
   - Parse markdown into AST
   - Extract title from first H1
   - Walk AST depth-first, building a flat text buffer and collecting style/paragraph requests
   - Strategy: first pass builds full plain text content with a single InsertText at index 1. Second pass applies styling (bold, italic, heading styles, links) using recorded offset ranges.
   - This two-pass approach avoids the offset-shifting problem entirely.
3. Handle nested inline styles (e.g., bold+italic, bold+link)
4. Handle list items — insert with bullet/number indicators, apply list paragraph styles
5. Handle horizontal rules — insert special character or styled paragraph

**Acceptance Criteria**:
- [ ] Headings H1-H6 produce correct `HEADING_1` through `HEADING_6` named styles
- [ ] Bold text produces `bold: true` text style
- [ ] Italic text produces `italic: true` text style
- [ ] Links produce `link: {url}` text style
- [ ] Ordered and unordered lists produce correct list structure
- [ ] Nested inline styles (bold+italic) work correctly
- [ ] Round-trip test: create doc from markdown, export, compare — core structure preserved
- [ ] Unit tests for each element type with specific offset assertions

**Files**: internal/convert/elements.go, internal/convert/md_to_docs.go, internal/convert/md_to_docs_test.go

---

### Task: local2gd-a2.3 - Wire conversion into gdrive.CreateDoc

Priority: 0
Labels: phase-3, logic, model:sonnet, complexity:medium

**Context**: Integration task — connect the converter output to the actual Google Docs API so we can create real docs from markdown.

**Implementation Steps**:
1. Create a high-level function `CreateDocFromMarkdown(client *gdrive.Client, folderID string, mdContent []byte) (gdrive.FileInfo, error)` in a new `internal/convert/pipeline.go`
2. Call `MarkdownToDocs` to get requests and title
3. Call `client.CreateDoc(folderID, title, requests)`
4. Similarly create `UpdateDocFromMarkdown(client *gdrive.Client, fileID string, mdContent []byte) error`
5. Write an integration test that creates a real doc and verifies it's accessible in Drive

**Acceptance Criteria**:
- [ ] `CreateDocFromMarkdown` with a sample markdown file creates a properly formatted Google Doc
- [ ] `UpdateDocFromMarkdown` replaces an existing doc's content with new markdown
- [ ] Created doc is viewable and editable by other Google users

**Files**: internal/convert/pipeline.go, internal/convert/pipeline_test.go

---

> **Checkpoint: Local markdown → real Google Doc works.** Can take a `.md` file and produce a properly formatted Google Doc.

---

## Epic: local2gd-a3 - local2gd-bidirectional-sync #4: Docs → Markdown Export

Priority: 0
Labels: phase-4, logic

Convert Google Docs back to clean markdown using Drive API export with post-processing.

### Task: local2gd-a3.1 - Implement markdown export post-processor

Priority: 0
Labels: phase-4, logic, model:sonnet, complexity:medium

**Context**: Google's `text/markdown` export has known issues (code blocks, image handling, whitespace). The post-processor cleans up the output.

**Implementation Steps**:
1. Create `internal/convert/docs_to_md.go`
2. Implement `PostProcessExport(rawMD []byte) []byte`:
   - Normalize trailing whitespace and line endings
   - Fix code block formatting if mangled (detect indented blocks that should be fenced)
   - Clean up image references (replace base64 data URIs with placeholder comments noting the image was stripped)
   - Normalize heading spacing (ensure blank line before/after headings)
   - Trim excessive blank lines (max 2 consecutive)
3. Write tests with known-bad export samples and expected cleaned output

**Acceptance Criteria**:
- [ ] Excessive whitespace is normalized
- [ ] Base64 image data URIs are replaced with clear placeholders
- [ ] Heading spacing is consistent
- [ ] Output is valid CommonMark + GFM markdown
- [ ] Tests cover each post-processing rule

**Files**: internal/convert/docs_to_md.go, internal/convert/docs_to_md_test.go

---

### Task: local2gd-a3.2 - Evaluate export quality with representative documents

Priority: 0
Labels: phase-4, logic, model:sonnet, complexity:medium, experiment

**Context**: The architecture flagged export quality as a key risk. Before building further, we need to validate that Google's markdown export + our post-processing is good enough.

**Implementation Steps**:
1. Create 5-10 representative Google Docs manually (prose with headings, lists, links, bold/italic, mixed formatting)
2. Export each using `gdrive.ExportMarkdown`
3. Run through `PostProcessExport`
4. Compare output to expected markdown
5. Run round-trip: original MD → create Doc → export → post-process → diff against original
6. Document findings: what works, what degrades, what's unacceptable
7. If quality is insufficient, document specific failure modes and create follow-up tasks for custom converter fallback

**Acceptance Criteria**:
- [ ] At least 5 representative documents tested
- [ ] Round-trip fidelity assessment documented (which elements survive, which degrade)
- [ ] Decision recorded: native export is sufficient for P0, or fallback needed
- [ ] Post-processor handles all observed issues from real exports

**Files**: internal/convert/docs_to_md.go, internal/convert/docs_to_md_test.go

---

> **Checkpoint: Bidirectional conversion validated.** MD→Doc and Doc→MD both work. Round-trip fidelity is documented and acceptable for MVP.

---

## Epic: local2gd-a4 - local2gd-bidirectional-sync #5: Sync Engine MVP

Priority: 0
Labels: phase-5, logic

The core sync loop: scan both sides, detect changes, execute actions, track state. This is where everything comes together.

### Task: local2gd-a4.1 - Implement local filesystem scanner

Priority: 0
Labels: phase-5, logic, model:sonnet, complexity:low

**Context**: The sync engine needs to know what markdown files exist locally and their content hashes.

**Implementation Steps**:
1. Create `internal/sync/scanner.go`
2. Define `LocalFile` struct: `RelPath string`, `AbsPath string`, `Hash string`, `ModTime time.Time`
3. Implement `ScanLocal(rootDir string) ([]LocalFile, error)` — walk directory tree, find all `.md` files, compute SHA-256 of each file's content, return sorted list
4. Skip `.local2gd/` directory during scan
5. Use `filepath.Rel` to get paths relative to the sync root

**Acceptance Criteria**:
- [ ] Returns all `.md` files recursively with correct relative paths
- [ ] SHA-256 hashes are deterministic for identical content
- [ ] `.local2gd/` directory is excluded from scan
- [ ] Handles empty directories and nested structures
- [ ] Tests with temp directory containing sample files

**Files**: internal/sync/scanner.go, internal/sync/scanner_test.go

---

### Task: local2gd-a4.2 - Implement remote Drive scanner

Priority: 0
Labels: phase-5, logic, model:sonnet, complexity:medium

**Context**: Mirror of the local scanner but for Google Drive. Lists all Google Docs in the remote folder and exports their markdown content for hashing.

**Implementation Steps**:
1. Add to `internal/sync/scanner.go`
2. Define `RemoteFile` struct: `RelPath string`, `DriveID string`, `Hash string`, `ModTime time.Time`
3. Implement `ScanRemote(client *gdrive.Client, folderID string) ([]RemoteFile, error)`:
   - Recursively list folder contents via `client.ListFolder`
   - For each Google Doc: export as markdown, compute SHA-256 of exported content
   - Build relative paths from folder hierarchy
   - Filter to only Google Docs MIME type (`application/vnd.google-apps.document`)
4. Handle nested folders by recursing with updated path prefix

**Acceptance Criteria**:
- [ ] Returns all Google Docs in folder tree with correct relative paths
- [ ] Hashes computed from exported markdown content
- [ ] Nested folders produce correct relative paths (e.g., `subdir/file.md`)
- [ ] Non-Doc files are skipped
- [ ] Integration test with real Drive folder

**Files**: internal/sync/scanner.go, internal/sync/scanner_test.go

---

### Task: local2gd-a4.3 - Implement state management

Priority: 0
Labels: phase-5, logic, model:sonnet, complexity:medium

**Context**: The sync engine needs persistent state to detect what changed since last sync. State lives in `.local2gd/state.json` within each synced folder.

**Implementation Steps**:
1. Create `internal/sync/state.go`
2. Define `SyncState` struct with version, remote folder ID, last sync time, and file map
3. Define `FileState` struct: DriveFileID, LocalHash, RemoteHash, LastSynced
4. Implement `LoadState(localRoot string) (*SyncState, error)` — read from `.local2gd/state.json`, return empty state if not found
5. Implement `SaveState(localRoot string, state *SyncState) error` — write JSON with indentation
6. Implement base copy management:
   - `SaveBase(localRoot, relPath string, content []byte) error` — write to `.local2gd/base/{relPath}`
   - `LoadBase(localRoot, relPath string) ([]byte, error)` — read base copy
   - `DeleteBase(localRoot, relPath string) error` — remove base copy
7. Create `.local2gd/` directory structure if it doesn't exist

**Acceptance Criteria**:
- [ ] State persists across process invocations
- [ ] Empty/missing state file handled gracefully (first sync scenario)
- [ ] Base copies written to correct paths mirroring local structure
- [ ] State file is valid JSON and human-readable (indented)
- [ ] Tests for load/save round-trip, missing file, corrupt file

**Files**: internal/sync/state.go, internal/sync/state_test.go

---

### Task: local2gd-a4.4 - Implement change classification

Priority: 0
Labels: phase-5, logic, model:opus, complexity:high

**Context**: The core diff logic: compare local files, remote files, and stored state to determine what action each file needs.

**Implementation Steps**:
1. Create `internal/sync/actions.go`
2. Define action types: `Push`, `Pull`, `CreateRemote`, `CreateLocal`, `DeleteRemote`, `DeleteLocal`, `Conflict`, `Unchanged`
3. Define `SyncAction` struct: `Type`, `RelPath`, `LocalFile *LocalFile`, `RemoteFile *RemoteFile`, `FileState *FileState`
4. Implement `ClassifyActions(local []LocalFile, remote []RemoteFile, state *SyncState) []SyncAction`:
   - Build maps by relative path for O(1) lookup
   - For each file in the union of local+remote+state:
     - In local only, not in state → `CreateRemote` (new local file)
     - In remote only, not in state → `CreateLocal` (new remote file)
     - In state only (not in local, not in remote) → `Unchanged` (both deleted, clean up state)
     - In local + state, not in remote → `DeleteLocal` (deleted remotely)
     - In remote + state, not in local → `DeleteRemote` (deleted locally)
     - In local + remote + state:
       - Local hash == stored local hash AND remote hash == stored remote hash → `Unchanged`
       - Local hash != stored local hash AND remote hash == stored remote hash → `Push`
       - Local hash == stored local hash AND remote hash != stored remote hash → `Pull`
       - Both changed → `Conflict`
     - In local + remote, not in state → `Conflict` (both sides have file, no base)
5. Sort actions: creates first, then pushes/pulls, then deletes, then conflicts

**Acceptance Criteria**:
- [ ] All 8 action types correctly classified
- [ ] New files on either side detected correctly
- [ ] Changes on one side only produce push/pull
- [ ] Changes on both sides produce conflict
- [ ] Deletions on one side propagate correctly
- [ ] Table-driven tests covering all classification scenarios
- [ ] Edge case: file exists on both sides but not in state (first sync for that file)

**Files**: internal/sync/actions.go, internal/sync/actions_test.go

---

### Task: local2gd-a4.5 - Implement sync engine execution loop

Priority: 0
Labels: phase-5, logic, model:opus, complexity:high

**Context**: Wire everything together: scan, classify, execute actions, update state. This is the main `sync.Run()` method.

**Implementation Steps**:
1. Create `internal/sync/engine.go`
2. Define `PairingConfig` struct: `Name`, `LocalDir`, `RemotePath`
3. Define `Report` struct: `Pushed`, `Pulled`, `Created`, `Deleted`, `Conflicts`, `Errors []error`
4. Implement `NewEngine(client *gdrive.Client, config PairingConfig) *Engine`
5. Implement `engine.Run(dryRun bool) (*Report, error)`:
   - Resolve remote path to folder ID
   - Scan local + remote
   - Load state
   - Classify actions
   - If dry-run: print plan and return
   - Execute each action:
     - `CreateRemote`: read local file → `CreateDocFromMarkdown` → update state + save base
     - `CreateLocal`: `ExportMarkdown` → `PostProcessExport` → write local file → update state + save base
     - `Push`: read local file → `UpdateDocFromMarkdown` → update state + save base
     - `Pull`: `ExportMarkdown` → `PostProcessExport` → write local file → update state + save base
     - `Conflict`: prompt user (pick local / pick remote / skip) → execute chosen action
     - `DeleteLocal`: delete local file → delete base → remove from state (P0: skip deletions, print warning)
     - `DeleteRemote`: `DeleteFile` → delete base → remove from state (P0: skip deletions, print warning)
   - Save state after each successful file action (crash resilience)
   - Print progress: `[1/15] Pushing design.md → Google Docs`
6. Conflict prompt: read from stdin, present options clearly

**Acceptance Criteria**:
- [ ] Full sync loop: scan → classify → execute → update state
- [ ] `--dry-run` prints plan without side effects
- [ ] New files created on both sides
- [ ] Changed files synced in correct direction
- [ ] Conflicts prompt user and execute chosen resolution
- [ ] State updated after each file (not just at end)
- [ ] Progress output shows what's happening
- [ ] Report summarizes results

**Files**: internal/sync/engine.go

---

### Task: local2gd-a4.6 - Wire sync engine into CLI command

Priority: 0
Labels: phase-5, setup, model:sonnet, complexity:medium

**Context**: Integration task — connect the sync engine to the `local2gd sync` cobra command, loading config and passing flags.

**Implementation Steps**:
1. Create `internal/sync/config.go` — implement config loading from viper/TOML
2. Update `cmd/sync.go`:
   - Load config file from XDG config dir
   - Get authorized client via `auth.Client()`
   - Create gdrive client
   - Create sync engine with first pairing from config
   - Pass `--dry-run` flag to `engine.Run()`
   - Print report summary
3. Create sample config file and document format
4. Handle missing config file with helpful error message

**Acceptance Criteria**:
- [ ] `local2gd sync` reads config, authenticates, and runs full sync
- [ ] `local2gd sync --dry-run` shows plan without executing
- [ ] Missing config prints: "No config found. Create ~/.config/local2gd/config.toml"
- [ ] Missing auth prints: "Not authenticated. Run `local2gd auth` first"
- [ ] End-to-end test: create config, create local files, run sync, verify files appear in Drive

**Files**: cmd/sync.go, internal/sync/config.go

---

> **Checkpoint: MVP sync works end-to-end.** User can auth, configure a pairing, and run `local2gd sync` to push/pull markdown files to/from Google Drive as native Google Docs.

---

## Epic: local2gd-a5 - local2gd-bidirectional-sync #6: Polish & Distribution

Priority: 1
Labels: phase-6, docs

Make the tool installable and usable by others.

### Task: local2gd-a5.1 - Improve error messages and user output

Priority: 1
Labels: phase-6, docs, model:sonnet, complexity:medium

**Context**: CLI tools live or die on their error messages. Transform technical errors into actionable guidance.

**Implementation Steps**:
1. Audit all error paths in auth, gdrive, sync packages
2. Wrap Google API errors with context: "Failed to list Drive folder 'Notes': {api error}. Check that the folder exists and you have access."
3. Add `slog` structured logging throughout — debug level for API calls, info for sync actions, warn for skipped files, error for failures
4. Add `--verbose` flag to root command that sets log level to debug
5. Format sync report as a clear summary table

**Acceptance Criteria**:
- [ ] Auth errors suggest remediation ("Token expired — run `local2gd auth` to re-authenticate")
- [ ] Drive API errors include the folder/file name in context
- [ ] `--verbose` shows detailed API call logging
- [ ] Sync report clearly shows: pushed, pulled, created, skipped, errored

**Files**: cmd/root.go, internal/auth/oauth.go, internal/auth/token.go, internal/gdrive/client.go, internal/sync/engine.go

---

### Task: local2gd-a5.2 - Set up goreleaser and distribution

Priority: 1
Labels: phase-6, setup, model:sonnet, complexity:medium

**Context**: Users need to install this tool easily. goreleaser handles cross-compilation, GitHub releases, and Homebrew.

**Implementation Steps**:
1. Create `.goreleaser.yaml` — build for darwin/amd64, darwin/arm64, linux/amd64, linux/arm64
2. Configure GitHub release with changelog generation
3. Configure Homebrew tap (create `homebrew-tap` repo or use inline tap config)
4. Add `Makefile` with `build`, `test`, `release` targets
5. Add GitHub Actions workflow for CI (test on push) and release (on tag)

**Acceptance Criteria**:
- [ ] `goreleaser build --snapshot` produces binaries for all target platforms
- [ ] `goreleaser release --snapshot` generates a complete release bundle
- [ ] CI workflow runs tests on push to main
- [ ] Release workflow triggers on `v*` tags

**Files**: .goreleaser.yaml, Makefile, .github/workflows/ci.yaml, .github/workflows/release.yaml

---

### Task: local2gd-a5.3 - Write README with usage examples

Priority: 1
Labels: phase-6, docs, model:sonnet, complexity:low

**Context**: Users need to understand how to install, configure, and use the tool.

**Implementation Steps**:
1. Create `README.md` with:
   - One-line description and motivation
   - Installation: `brew install`, `go install`, GitHub releases
   - Quick start: auth → create config → sync
   - Config file format with examples
   - Command reference: `auth`, `sync`, `sync --dry-run`
   - Known limitations (which markdown elements degrade on round-trip)
   - Contributing section
2. Add `.gitignore` with recommendation to add `.local2gd/` to project gitignores

**Acceptance Criteria**:
- [ ] New user can follow README from zero to working sync
- [ ] Config format documented with examples
- [ ] Known limitations clearly stated
- [ ] Installation instructions for all distribution methods

**Files**: README.md, .gitignore

---

> **Checkpoint: Tool is distributable.** Someone can `brew install`, configure, and sync.

---

## Epic: local2gd-a6 - local2gd-bidirectional-sync #7: Multiple Pairings

Priority: 1
Labels: phase-7, logic, p1

Support multiple local↔Drive folder pairings in a single config.

### Task: local2gd-a6.1 - Implement multiple pairing config and CLI selection

Priority: 1
Labels: phase-7, logic, model:sonnet, complexity:medium

**Context**: Users typically have multiple folder sets to sync (notes, work docs, project docs). The config already supports multiple pairings — the CLI needs to support selecting them.

**Implementation Steps**:
1. Update `internal/sync/config.go` to load all pairings from config
2. Update `cmd/sync.go`:
   - `local2gd sync` with no args → sync all pairings sequentially
   - `local2gd sync notes` → sync only the named pairing
   - `local2gd sync --dry-run notes` → dry-run a specific pairing
3. Print clear separator between pairings in output
4. If a pairing fails, continue with remaining pairings and report errors at the end

**Acceptance Criteria**:
- [ ] `local2gd sync` syncs all configured pairings
- [ ] `local2gd sync notes` syncs only the "notes" pairing
- [ ] Invalid pairing name prints error with list of valid names
- [ ] Failure in one pairing doesn't block others
- [ ] Summary report covers all pairings

**Files**: cmd/sync.go, internal/sync/config.go

---

## Epic: local2gd-a7 - local2gd-bidirectional-sync #8: Three-Way Merge

Priority: 1
Labels: phase-8, logic, p1

Upgrade conflict resolution from "pick one" to automatic three-way merge using stored base versions.

### Task: local2gd-a7.1 - Implement three-way merge via git merge-file

Priority: 1
Labels: phase-8, logic, model:sonnet, complexity:medium

**Context**: When both sides change the same file, three-way merge can often auto-resolve if the changes don't overlap. This uses the base copy stored at last sync as the common ancestor.

**Implementation Steps**:
1. Create `internal/sync/merge.go`
2. Implement `ThreeWayMerge(base, local, remote []byte) (merged []byte, hasConflicts bool, err error)`:
   - Write base, local, remote to temp files
   - Run `git merge-file -p --diff3 local base remote`
   - Exit code 0 → clean merge, return merged content
   - Exit code 1 → conflicts, return merged content with conflict markers, `hasConflicts = true`
   - Exit code >1 → error
3. Check for `git` availability at startup, fall back to file-level pick-one if not found
4. Write tests with known merge scenarios (clean merge, conflicting merge, one-side-only change)

**Acceptance Criteria**:
- [ ] Non-overlapping changes merge cleanly
- [ ] Overlapping changes produce conflict markers in output
- [ ] Missing `git` binary falls back gracefully to pick-one
- [ ] Tests cover: clean merge, conflict, single-side change, empty file

**Files**: internal/sync/merge.go, internal/sync/merge_test.go

---

### Task: local2gd-a7.2 - Integrate three-way merge into sync engine

Priority: 1
Labels: phase-8, logic, model:sonnet, complexity:medium

**Context**: Replace the "pick one" conflict resolution with three-way merge. Fall back to pick-one only when merge fails or user rejects merged result.

**Implementation Steps**:
1. Update conflict handling in `internal/sync/engine.go`:
   - Load base copy for the conflicted file
   - If base exists: attempt `ThreeWayMerge(base, local, remote)`
   - If clean merge: apply merged result to both sides, update state
   - If merge has conflicts: show conflict summary, prompt user: accept merged (with markers), pick local, pick remote, skip
   - If no base: fall back to pick-one (can't three-way merge without ancestor)
2. Update report to show merge statistics (auto-merged, conflict-merged, pick-one)

**Acceptance Criteria**:
- [ ] Non-overlapping concurrent edits auto-merge without user prompt
- [ ] Conflicting edits show conflict markers and prompt for resolution
- [ ] Files without base copies fall back to pick-one
- [ ] Sync report distinguishes auto-merged from manually resolved conflicts

**Files**: internal/sync/engine.go

---

## Epic: local2gd-a8 - local2gd-bidirectional-sync #9: Deletion & Soft-Delete

Priority: 1
Labels: phase-9, logic, p1

Propagate deletions across sides with a safety buffer.

### Task: local2gd-a8.1 - Implement soft-delete trash buffer

Priority: 1
Labels: phase-9, logic, model:sonnet, complexity:medium

**Context**: Deletion propagation is dangerous — an accidental local delete shouldn't immediately nuke the Google Doc. Soft-delete moves files to a local trash buffer before propagating.

**Implementation Steps**:
1. Create `internal/sync/trash.go`
2. Implement `MoveToTrash(localRoot, relPath string) error` — move file to `.local2gd/trash/{relPath}` with timestamp suffix (e.g., `design.md.2026-03-25T103000`)
3. Implement `CleanTrash(localRoot string, maxAge time.Duration) error` — delete files older than maxAge (default 30 days)
4. Implement `ListTrash(localRoot string) ([]TrashEntry, error)` — list trashed files with dates
5. Run `CleanTrash` at the start of every sync operation

**Acceptance Criteria**:
- [ ] Deleted files are preserved in trash with timestamp
- [ ] Files older than 30 days are cleaned up
- [ ] Trash doesn't interfere with sync scanning
- [ ] Tests for trash, cleanup, and listing

**Files**: internal/sync/trash.go, internal/sync/trash_test.go

---

### Task: local2gd-a8.2 - Wire deletion propagation into sync engine

Priority: 1
Labels: phase-9, logic, model:sonnet, complexity:medium

**Context**: Enable the `DeleteLocal` and `DeleteRemote` actions that were deferred in the P0 sync engine (which just printed warnings).

**Implementation Steps**:
1. Update `internal/sync/engine.go` to handle deletion actions:
   - `DeleteLocal` (file deleted remotely): `MoveToTrash` the local file → remove from state → delete base copy
   - `DeleteRemote` (file deleted locally): `client.DeleteFile` (Drive trash) → remove from state → delete base copy
2. Add `--no-delete` flag to skip deletion propagation entirely
3. In dry-run mode, clearly mark deletions with a warning: `[DELETE] design.md (deleted remotely, will be trashed locally)`

**Acceptance Criteria**:
- [ ] Remote deletion → local file moved to `.local2gd/trash/`
- [ ] Local deletion → Drive file moved to Drive trash
- [ ] `--no-delete` flag prevents all deletion propagation
- [ ] Dry-run clearly shows planned deletions
- [ ] State and base copies cleaned up after deletion

**Files**: internal/sync/engine.go

---

## Epic: local2gd-a9 - local2gd-bidirectional-sync #10: Status & Diff Commands

Priority: 1
Labels: phase-10, logic, p1

Read-only commands to inspect sync state without modifying anything.

### Task: local2gd-a9.1 - Implement status command

Priority: 1
Labels: phase-10, logic, model:sonnet, complexity:medium

**Context**: Users need to see what's changed before running sync. `local2gd status` scans and classifies but doesn't execute.

**Implementation Steps**:
1. Create `cmd/status.go` — new cobra subcommand
2. Reuse sync engine's scan + classify logic without the execute step
3. Output grouped by action type:
   ```
   notes (~/Documents/notes ↔ Notes)
     New locally:    2 files
     New remotely:   1 file
     Modified locally: 3 files
     Modified remotely: 0 files
     Conflicts:      1 file
     Unchanged:      15 files
   ```
4. With `--verbose`: list individual files per category

**Acceptance Criteria**:
- [ ] `local2gd status` shows change summary without modifying anything
- [ ] `local2gd status notes` shows status for a specific pairing
- [ ] `--verbose` lists individual files
- [ ] Works with multiple pairings (shows all)

**Files**: cmd/status.go

---

### Task: local2gd-a9.2 - Implement diff command

Priority: 1
Labels: phase-10, logic, model:sonnet, complexity:medium

**Context**: Users want to see the actual content differences before syncing, not just which files changed.

**Implementation Steps**:
1. Create `cmd/diff.go` — new cobra subcommand
2. For each changed file: compute unified diff between local version and remote export (or between current and base)
3. Use a Go diff library (e.g., `sergi/go-diff`) for unified diff output
4. `local2gd diff` — all changed files across all pairings
5. `local2gd diff path/to/file.md` — specific file
6. Color output if terminal supports it

**Acceptance Criteria**:
- [ ] `local2gd diff` shows unified diffs for all changed files
- [ ] `local2gd diff file.md` shows diff for specific file
- [ ] Diffs are clear and readable
- [ ] No side effects (read-only operation)

**Files**: cmd/diff.go

---

## Epic: local2gd-a10 - local2gd-bidirectional-sync #11: Sidecar Metadata & Frontmatter

Priority: 1
Labels: phase-11, logic, p1

Preserve Google Doc metadata and markdown frontmatter through sync cycles.

### Task: local2gd-a10.1 - Implement sidecar metadata storage

Priority: 1
Labels: phase-11, logic, model:sonnet, complexity:medium

**Context**: Google Docs have metadata (comments, permissions, properties) that can't be represented in markdown. Store in sidecar JSON files so it's not lost.

**Implementation Steps**:
1. Create `internal/sync/sidecar.go`
2. Define `DocMetadata` struct: Comments, Permissions, Properties, Title, LastModifiedBy, CreatedTime
3. Implement `FetchMetadata(client *gdrive.Client, fileID string) (*DocMetadata, error)` — use Drive API to get file metadata, use Docs API to get comments
4. Implement `SaveSidecar(localRoot, relPath string, meta *DocMetadata) error` — write to `.local2gd/meta/{relPath}.json`
5. Implement `LoadSidecar(localRoot, relPath string) (*DocMetadata, error)` — read sidecar
6. Integrate into sync engine: fetch and save sidecar on every pull/create-local action

**Acceptance Criteria**:
- [ ] Sidecar files created in `.local2gd/meta/` directory
- [ ] Comments, permissions, and properties preserved in JSON
- [ ] Sidecar updated on each sync
- [ ] Missing sidecar handled gracefully (first sync)

**Files**: internal/sync/sidecar.go, internal/sync/sidecar_test.go

---

### Task: local2gd-a10.2 - Implement frontmatter preservation

Priority: 1
Labels: phase-11, logic, model:sonnet, complexity:medium

**Context**: Markdown files often have YAML frontmatter (used by Obsidian, Hugo, Jekyll). This must survive round-trips — strip before uploading to Docs, re-attach on export.

**Implementation Steps**:
1. Create `internal/convert/frontmatter.go`
2. Implement `StripFrontmatter(md []byte) (body []byte, frontmatter []byte, error)` — detect `---` delimited YAML block at start of file, return body and raw frontmatter separately
3. Implement `AttachFrontmatter(body []byte, frontmatter []byte) []byte` — prepend frontmatter to body
4. Integrate into conversion pipeline:
   - MD → Docs: strip frontmatter before converting, store frontmatter in state or sidecar
   - Docs → MD: re-attach stored frontmatter to exported markdown
5. Store frontmatter in state.json per file (small, changes rarely)

**Acceptance Criteria**:
- [ ] Frontmatter stripped before Docs upload (Docs don't show raw YAML)
- [ ] Frontmatter re-attached on export (preserves file compatibility with Obsidian/Hugo)
- [ ] Files without frontmatter work normally
- [ ] Round-trip: MD with frontmatter → Doc → MD → frontmatter preserved exactly
- [ ] Tests cover: with frontmatter, without, with empty frontmatter

**Files**: internal/convert/frontmatter.go, internal/convert/frontmatter_test.go, internal/convert/pipeline.go

---

### Task: local2gd-a10.3 - Implement Doc title from heading

Priority: 1
Labels: phase-11, logic, model:haiku, complexity:low

**Context**: When creating a Google Doc from markdown, the Doc title should come from the first `# heading` rather than the filename. This makes Docs more readable for collaborators.

**Implementation Steps**:
1. Update `internal/convert/pipeline.go`:
   - `CreateDocFromMarkdown` already extracts title from `MarkdownToDocs`
   - If title is empty (no H1), derive from filename: `design-notes.md` → `Design Notes`
2. On push/update: if the Doc title differs from the current H1, update the Doc title via Drive API `files.update`

**Acceptance Criteria**:
- [ ] New Doc created with title from first `# heading`
- [ ] Missing H1 falls back to humanized filename
- [ ] Title updates when H1 changes on subsequent syncs

**Files**: internal/convert/pipeline.go

---

> **Checkpoint: Full P1 feature set complete.** Multiple pairings, three-way merge, soft-delete, status/diff commands, sidecar metadata, frontmatter preservation, Doc titles from headings.

---

## Summary

| Epic | Tasks | Priority | Description |
|------|-------|----------|-------------|
| local2gd-a0 | 4 | P0 | Skeleton + Auth |
| local2gd-a1 | 2 | P0 | Google Drive Client |
| local2gd-a2 | 3 | P0 | Markdown → Docs Conversion |
| local2gd-a3 | 2 | P0 | Docs → Markdown Export |
| local2gd-a4 | 6 | P0 | Sync Engine MVP |
| local2gd-a5 | 3 | P1 | Polish & Distribution |
| local2gd-a6 | 1 | P1 | Multiple Pairings |
| local2gd-a7 | 2 | P1 | Three-Way Merge |
| local2gd-a8 | 2 | P1 | Deletion & Soft-Delete |
| local2gd-a9 | 2 | P1 | Status & Diff Commands |
| local2gd-a10 | 3 | P1 | Sidecar Metadata & Frontmatter |

**Total**: 11 epics, 30 tasks
