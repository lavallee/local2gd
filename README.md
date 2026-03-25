# local2gd

Bidirectional sync between local markdown files and Google Docs.

Write in markdown locally. Collaborators see native Google Docs. Changes sync both ways.

## Install

```bash
# From source
go install github.com/lavallee/local2gd@latest

# Or build locally
git clone https://github.com/lavallee/local2gd.git
cd local2gd
make build
```

## Quick Start

### 1. Authenticate

```bash
local2gd auth
```

Opens your browser to authorize access to Google Drive and Docs.

### 2. Configure

Create `~/.config/local2gd/config.toml`:

```toml
[pairings.notes]
local = "~/Documents/notes"
remote = "Notes"

[pairings.work]
local = "~/Documents/work-docs"
remote = "Engineering/Docs"
```

- `local` — path to your local markdown folder
- `remote` — path in Google Drive (relative to My Drive root)

### 3. Sync

```bash
# Preview what would change
local2gd sync --dry-run

# Sync all pairings
local2gd sync

# Sync a specific pairing
local2gd sync notes
```

## Commands

| Command | Description |
|---------|-------------|
| `local2gd auth` | Authenticate with Google |
| `local2gd auth --force` | Re-authenticate |
| `local2gd sync` | Sync all configured pairings |
| `local2gd sync <name>` | Sync a specific pairing |
| `local2gd sync --dry-run` | Preview changes without syncing |
| `local2gd sync --no-delete` | Skip deletion propagation |
| `local2gd status` | Show what's changed since last sync |
| `local2gd diff` | Show content diffs for changed files |
| `local2gd --verbose` | Enable debug logging |

## How It Works

- **Local → Drive**: Markdown is parsed and converted to Google Docs using the Docs API `batchUpdate` (headings, bold, italic, links, lists).
- **Drive → Local**: Google Docs are exported as markdown via the Drive API's `text/markdown` export, then post-processed for consistency.
- **Change detection**: SHA-256 content hashes compared against stored state from the last sync.
- **Conflicts**: When both sides change the same file, you're prompted to pick local or remote (or skip).

State is stored in `.local2gd/` within each synced local folder.

## Supported Markdown Elements

| Element | Local → Docs | Docs → Local |
|---------|:---:|:---:|
| Headings (H1-H6) | Yes | Yes |
| Bold | Yes | Yes |
| Italic | Yes | Yes |
| Links | Yes | Yes |
| Ordered lists | Yes | Yes |
| Unordered lists | Yes | Yes |
| Horizontal rules | Yes | Yes |
| Paragraphs | Yes | Yes |
| Code blocks | Partial | Partial |
| Tables | Not yet | Not yet |
| Images | Not yet | Not yet |

## Known Limitations

- **Code blocks** may lose language hints on round-trip through Google Docs.
- **Images** are stripped from Google Docs exports (replaced with placeholder comments).
- **Google Docs comments and suggestions** are not represented in markdown.
- **Complex table formatting** (merged cells, colors) has no markdown equivalent.
- **Frontmatter** (YAML) is preserved through sync but not visible in Google Docs.

## Configuration

Config file location follows the [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html):

- Config: `$XDG_CONFIG_HOME/local2gd/config.toml` (default: `~/.config/local2gd/config.toml`)
- Auth tokens: `$XDG_DATA_HOME/local2gd/token.json` (default: `~/.local/share/local2gd/token.json`)
- Sync state: `.local2gd/` directory within each synced local folder

## Google Cloud Setup

To use local2gd, you need a Google Cloud project with the Drive and Docs APIs enabled:

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project
3. Enable the **Google Drive API** and **Google Docs API**
4. Configure the OAuth consent screen (External, add your email as a test user)
5. Create **OAuth 2.0 credentials** → Desktop application
6. Configure credentials via environment variables or config file:

**Option A: Environment variables**
```bash
export LOCAL2GD_CLIENT_ID=your-id.apps.googleusercontent.com
export LOCAL2GD_CLIENT_SECRET=your-secret
```

**Option B: Config file** (`~/.config/local2gd/config.toml`)
```toml
[auth]
client_id = "your-id.apps.googleusercontent.com"
client_secret = "your-secret"

[pairings.notes]
local = "~/Documents/notes"
remote = "Notes"
```

## License

MIT
