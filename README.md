# TetherSSH - SSH Terminal Emulator

A Go-based SSH terminal emulator with session management, built on Fyne 2 GUI framework and the gopyte terminal emulation library.

**The first fully-functional Fyne-based SSH terminal with session management, scrollback history, and text selection.**

## Screenshots

### Session Manager with Tree View
![TetherSSH Session Manager](screenshots/session_manager.png)

*Hierarchical folder/session tree with search filter, connection status indicators, and multi-tab interface*

### Full-Screen Applications
![TetherSSH running htop](screenshots/htop.png)

*htop running with full 256-color support, proper resize handling, and alternate screen buffer*

### Session Editor
![TetherSSH Session Editor](screenshots/session_detail.png)

*CRUD interface for managing sessions and folders with full authentication configuration*

---

## Current Status: Alpha (v0.2.0)

**Working:**
- SSH connectivity with password and public key authentication
- Multi-tab terminal interface with tree-based session navigator
- Full terminal emulation via gopyte (VT100/ANSI parsing)
- Keyboard input routing to SSH sessions
- Dynamic terminal resize with proper SSH window-change signaling
- Full-screen applications (htop, vim, nano, etc.)
- 256-color support with proper color mapping
- Scrollback history (1000+ lines) with mouse wheel scrolling
- Text selection with clipboard support (double-click word, triple-click line)
- **Tree-based session navigator** with collapsible folders
- **Session persistence** via YAML configuration
- **Session search/filter** for quick access to devices
- **Session editor** with full CRUD operations
- **Quick Connect** dialog for ad-hoc connections
- **SSH key authentication** with encrypted key support

**Known Issues:**
- UI freezes on application close (cleanup race condition) - partially fixed
- SSH agent support not fully implemented
- Host key verification not yet implemented

---

## Key Features

### Terminal Emulation
TetherSSH uses **gopyte**, a custom terminal emulation library written specifically for this project. Unlike the Fyne project's proof-of-concept terminal, gopyte provides:

- **Full history buffer** with configurable scrollback (default 1000 lines)
- **Wide character support** for CJK characters and emojis
- **Alternate screen buffer** for full-screen applications (vim, htop, less)
- **Proper resize handling** that syncs local screen, gopyte buffers, and SSH session

### Session Management
- **Tree-based navigator** with collapsible folders and session counts
- **Real-time search/filter** - instantly find sessions by name, host, group, or device type
- Multiple concurrent SSH connections in tabs
- Visual connection status indicators (●=connected, ○=connecting, ✗=error)
- One-click connect with automatic credential handling

### Authentication Support
- **Password authentication** - prompted on connect or stored in session
- **SSH key authentication** - supports RSA, ECDSA, Ed25519 keys
- **Encrypted key support** - passphrase prompts for protected keys
- **Keyboard-interactive** - for MFA/RADIUS environments
- Configurable per-session authentication type

### Quick Connect
Press the ⏩ button for ad-hoc connections without saving a session:
- Enter host, port, username
- Choose Password or SSH Key authentication
- Default key path: `~/.ssh/id_rsa`
- Optional key passphrase for encrypted keys

### Session Editor
Press the ⚙️ button to open the full session manager:
- **Folders panel** - organize sessions into groups
- **Sessions panel** - view/edit sessions in selected folder
- **Add/Edit/Delete** - full CRUD operations
- **Import** - load sessions from other YAML files
- **Export** - save current configuration
- Device metadata fields (type, vendor, model) for network equipment

### Session Persistence
Sessions are stored in `./sessions/sessions.yaml` (relative to app directory):

```yaml
# TetherSSH Sessions File
# Auth types: password, publickey, keyboard-interactive

- folder_name: Production
  sessions:
    - display_name: web-server-01
      host: 10.0.1.10
      port: "22"
      username: admin
      auth_type: publickey
      key_path: ~/.ssh/id_rsa
      DeviceType: linux

- folder_name: Lab
  sessions:
    - display_name: cisco-router
      host: 172.16.1.1
      port: "22"
      username: admin
      auth_type: password
      DeviceType: cisco_ios
      Vendor: Cisco
```

---

## Architecture Overview

### Component Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         main.go                                  │
│  - Application entry point                                       │
│  - Window creation                                               │
│  - Theme setup                                                   │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                     SessionManager                               │
│  - Tree-based session navigator (20% width)                     │
│  - Search/filter box                                            │
│  - DocTabs container for terminals (80% width)                  │
│  - Keyboard event capture and forwarding                        │
│  - Connection orchestration                                      │
└─────────────────────────────────────────────────────────────────┘
         │                      │                       │
         ▼                      ▼                       ▼
┌─────────────────┐   ┌─────────────────┐   ┌─────────────────────┐
│  SessionStore   │   │  SessionEditor  │   │    SessionTab       │
│  - YAML load    │   │  - CRUD modal   │   │  - SSHTerminalWidget│
│  - YAML save    │   │  - Folder mgmt  │   │  - Tab reference    │
│  - Tree data    │   │  - Import/Export│   │  - Connection state │
└─────────────────┘   └─────────────────┘   └─────────────────────┘
                                                      │
                                                      ▼
┌─────────────────────────────────────────────────────────────────┐
│                    SSHTerminalWidget                             │
│  - Embeds NativeTerminalWidget                                  │
│  - Owns SSHBackend                                              │
│  - Sets writeOverride to route keyboard → SSH                   │
│  - Sets onResizeCallback for resize propagation                 │
│  - sshReadLoop() feeds SSH output → terminal emulator           │
│  - triggerPostConnectResize() syncs size after connection       │
└─────────────────────────────────────────────────────────────────┘
          │                                    │
          ▼                                    ▼
┌─────────────────────────┐   ┌─────────────────────────────────┐
│      SSHBackend         │   │     NativeTerminalWidget        │
│  - golang.org/x/crypto  │   │  - gopyte WideCharScreen        │
│  - SSH client/session   │   │  - Fyne TextGrid renderer       │
│  - PTY request          │   │  - Keyboard/mouse handling      │
│  - WindowChange resize  │   │  - History/scrollback           │
│  - Keepalive            │   │  - writeOverride func field     │
│  - Auth chain           │   │  - onResizeCallback func field  │
└─────────────────────────┘   └─────────────────────────────────┘
```

### File Structure

```
tetherssh/
├── main.go                  # Application entry, window setup
├── session_manager.go       # Tree navigator, search, tab management
├── session_persistence.go   # YAML load/save, SessionStore
├── session_editor.go        # CRUD modal dialog
├── ssh_backend.go           # SSH client, auth chain, SSHTerminalWidget
├── terminal_widget.go       # NativeTerminalWidget - core terminal UI
├── terminal_pty.go          # Local PTY support, WriteToPTY, history
├── terminal_events.go       # Keyboard/mouse event handling, resize
├── terminal_display.go      # TextGrid rendering, viewport calculation
├── terminal_selection.go    # Text selection and clipboard
├── theme.go                 # Fyne theme, color mappings
├── scroll_container.go      # Custom scroll container
├── screenshots/             # Application screenshots
│   ├── htop.png
│   ├── session_manager.png
│   └── session_detail.png
└── internal/gopyte/         # Terminal emulation library
    ├── screen.go            # Base screen buffer (NativeScreen)
    ├── history_screen.go    # Scrollback history management
    ├── wide_char_screen.go  # Wide character & alternate screen support
    └── streams.go           # ANSI escape sequence parser
```

---

## Key Design Decisions

### 1. writeOverride Pattern

Instead of complex interface gymnastics, we use a simple function field override:

```go
// In NativeTerminalWidget
type NativeTerminalWidget struct {
    writeOverride    func([]byte)        // Set by SSHTerminalWidget
    onResizeCallback func(cols, rows int) // Set by SSHTerminalWidget
}

func (t *NativeTerminalWidget) WriteToPTY(data []byte) error {
    if t.writeOverride != nil {
        t.writeOverride(data)  // Routes to SSH
        return nil
    }
    // Fall back to local PTY
    return t.ptyManager.Write(data)
}
```

This avoids Go's embedding limitations where method calls on embedded types don't dispatch to wrapper methods.

### 2. Resize Callback Pattern

Go's struct embedding doesn't support virtual method dispatch. When `NativeTerminalWidget.performResize()` is called, it can't know to call `SSHTerminalWidget.ResizeTerminal()`. The solution:

```go
// In NewSSHTerminalWidget
w.NativeTerminalWidget.SetResizeCallback(func(cols, rows int) {
    w.ResizeTerminal(cols, rows)
})

// In NativeTerminalWidget.performResize()
if t.onResizeCallback != nil {
    t.onResizeCallback(newCols, newRows)  // SSH gets the resize
} else {
    t.performPTYResize(newRows, newCols)  // Local PTY resize
}
```

### 3. Tree-Based Session Navigation

The session navigator uses Fyne's `widget.Tree` with a node ID scheme:

```go
// Node ID format:
// ""              = root
// "folder:Lab"    = folder node
// "session:Lab-0" = session leaf node

treeData    map[string][]string      // parent -> children mapping
sessionByID map[string]*SessionInfo  // quick lookup by node ID
```

This enables collapsible folders, session counts, and efficient filtering while maintaining the hierarchical structure.

### 4. Auth Type Persistence

Sessions store authentication preferences in YAML:

```go
type SessionYAML struct {
    Username      string `yaml:"username,omitempty"`
    AuthType      string `yaml:"auth_type,omitempty"`      // password, publickey, keyboard-interactive
    KeyPath       string `yaml:"key_path,omitempty"`
    KeyPassphrase string `yaml:"key_passphrase,omitempty"`
}
```

The `connectToSession()` method routes based on auth type:
- `publickey` → Direct connect (passphrase prompted if needed)
- `password` → Prompt for password if not stored
- `keyboard-interactive` → Let SSH handle prompts

---

## gopyte Terminal Emulation

gopyte is a terminal emulation library built specifically for TetherSSH. It provides:

### Screen Hierarchy
```
NativeScreen (base)
    └── HistoryScreen (adds scrollback)
        └── WideCharScreen (adds wide chars + alternate screen)
```

### Key Features
- **VT100/ANSI parsing** via Stream.Feed()
- **Scrollback history** with linked list storage
- **Alternate screen buffer** for vim, htop, less, etc.
- **Wide character support** for CJK and emojis
- **Resize handling** that preserves content and history

### Escape Sequences Supported
- Cursor movement (CUP, CUU, CUD, CUF, CUB)
- Erase operations (ED, EL)
- SGR attributes (colors, bold, underline, etc.)
- Scroll regions (DECSTBM)
- Alternate screen (DECSET/DECRST 1049)
- Window title (OSC 0, 1, 2)

---

## Authentication Flow

### Password Authentication
```
User clicks Connect
    → connectToSession() checks auth type
    → promptPasswordAndConnect() shows dialog
    → User enters password, clicks Connect
    → doConnect(session, password)
    → SSHBackend.Connect() with password
    → SSH handshake completes
    → State → Connected
```

### Public Key Authentication
```
User clicks Connect
    → connectToSession() sees AuthPublicKey
    → doConnect(session, "") [no password needed]
    → SSHBackend.buildAuthMethods()
    → getPublicKeyAuth() loads key from KeyPath
    → If encrypted: authPromptHandler asks for passphrase
    → ssh.PublicKeys(signer) added to auth chain
    → SSH handshake completes
    → State → Connected
```

### Authentication Chain
Methods are tried in order:
1. SSH Agent (if `UseAgent=true` and `SSH_AUTH_SOCK` available)
2. Public Key (from `PrivateKeyPath` or `PrivateKey` bytes)
3. Password (`ssh.Password` method)
4. Keyboard-Interactive (fallback for MFA/RADIUS)

---

## Roadmap

### Phase 1: Core Features ✅ Complete
- [x] Fix vim/htop resize issues
- [x] Implement resize callback pattern
- [x] Post-connect resize sync
- [x] Buffer bounds safety in gopyte
- [x] Session persistence (YAML config)
- [x] Public key authentication
- [x] Encrypted key passphrase support
- [x] Session search/filter
- [x] Session editor with CRUD
- [x] Quick Connect dialog
- [x] Tree-based session navigator

### Phase 2: Stability (Current → v0.3)
- [ ] Fix application close freeze completely
- [ ] Implement SSH agent support
- [ ] Add host key verification with known_hosts
- [ ] Clean up debug logging
- [ ] Cross-platform testing (Windows, Linux, macOS)

### Phase 3: Logging & Security (v0.3 → v0.4)
- [ ] Per-session logging (raw/timestamped output to file)
- [ ] Log viewer/browser
- [ ] Encrypted credentials manager (AES-256-GCM)
- [ ] Master password unlock flow
- [ ] Credential references in sessions (decouple passwords from YAML)

### Phase 4: Terminal Features (v0.4 → v0.5)
- [ ] Split panes (horizontal/vertical)
- [ ] Find in terminal output
- [ ] Clickable URLs
- [ ] Command snippets/macros

### Phase 5: Advanced (v0.5 → v1.0)
- [ ] SFTP file browser integration
- [ ] Port forwarding UI
- [ ] Jump host/proxy support
- [ ] Session import from PuTTY/SecureCRT

---

## Building

### Linux
```bash
go build -o tetherssh .
./tetherssh
```

### Windows
```bash
GOOS=windows GOARCH=amd64 go build -o tetherssh.exe .
```

### macOS
```bash
GOOS=darwin GOARCH=amd64 go build -o tetherssh-macos .
```

### Dependencies
```
fyne.io/fyne/v2               # GUI framework
golang.org/x/crypto/ssh       # SSH client
github.com/creack/pty         # Local PTY (Unix)
github.com/mattn/go-runewidth # Wide character width calculation
github.com/google/uuid        # Tab/session unique IDs
gopkg.in/yaml.v3              # Session persistence
```

---

## Configuration

### Session File Location
TetherSSH looks for sessions in `./sessions/sessions.yaml` relative to the application directory. If not found, a stub file is created with example entries.

### Supported Auth Types
| YAML Value | Description |
|------------|-------------|
| `password` | Prompt for password on connect |
| `publickey` | Use SSH key from `key_path` |
| `keyboard-interactive` | MFA/RADIUS environments |

### Key Path Expansion
The `~` character is expanded to the user's home directory:
- `~/.ssh/id_rsa` → `/home/username/.ssh/id_rsa`
- `~/.ssh/id_ed25519` → `/home/username/.ssh/id_ed25519`

---

## Lessons Learned

### Go-Specific Challenges
1. **Embedded types don't override methods** - Use function fields (callbacks) instead
2. **Fyne threading model** - Use `fyne.Do()` for UI updates from goroutines
3. **Buffer vs logical dimensions** - After resize, always check `len(buffer)` not `screen.lines`

### Terminal Emulation
1. **Alternate screen buffer** - Essential for vim, htop, less to work correctly
2. **Resize timing** - Must resize gopyte screen BEFORE sending SSH WindowChange
3. **History preservation** - Resize should preserve scrollback content

### Architecture
1. **Callbacks over inheritance** - Go's composition model requires explicit callback wiring
2. **Separation of concerns** - SSHBackend knows nothing about UI
3. **Defensive coding** - Bounds check everything, especially after resize
4. **Tree data structures** - Use maps for parent→children relationships, separate lookup maps for node data

---

## Related Projects

- **[Secure Cartography](https://github.com/scottpeterman/secure_cartography)** - Network discovery and topology mapping (134+ GitHub stars)
- **[TerminalTelemetry](https://pypi.org/project/terminaltelemetry/)** - PyQt6-based SSH terminal with real-time monitoring
- **[VelociTerm](https://github.com/scottpeterman/velociterm)** - Web-based SSH terminal with NetBox integration

---

## License

MIT License - See LICENSE file

---

*Last updated: November 29, 2025*
*Author: Scott Peterman*