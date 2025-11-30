// session_manager.go - Session manager for tabbed SSH terminal app
// Uses Tree widget for hierarchical folder/session display
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// SessionInfo holds metadata about a saved session
type SessionInfo struct {
	ID            string
	Name          string
	Host          string
	Port          int
	Username      string
	Password      string
	AuthType      AuthMethod
	KeyPath       string
	KeyPassphrase string
	Group         string
	
	DeviceType string
	Vendor     string
	Model      string
	CredsID    string
}

// SessionManager manages multiple terminal sessions
type SessionManager struct {
	// UI components
	window        fyne.Window
	sessionTree   *widget.Tree
	tabContainer  *container.DocTabs
	mainContainer *fyne.Container
	searchEntry   *widget.Entry
	
	// Session data
	savedSessions    []SessionInfo
	filteredSessions []SessionInfo
	filterText       string
	activeTabs       map[string]*SessionTab
	tabsMutex        sync.RWMutex
	
	// Tree data structures
	treeData      map[string][]string // parent -> children mapping
	sessionByID   map[string]*SessionInfo // quick lookup by tree node ID
	
	// Session persistence
	sessionStore *SessionStore
	
	// Currently selected in tree
	selectedNodeID  string
	selectedSession *SessionInfo
	
	// Callbacks
	onSessionConnect func(session SessionInfo)
}

// SessionTab represents an active terminal tab
type SessionTab struct {
	TabID    string
	Info     SessionInfo
	Terminal *SSHTerminalWidget
	Tab      *container.TabItem
	State    ConnectionState
}

// NewSessionManager creates a new session manager
func NewSessionManager(window fyne.Window) *SessionManager {
	sm := &SessionManager{
		window:      window,
		activeTabs:  make(map[string]*SessionTab),
		treeData:    make(map[string][]string),
		sessionByID: make(map[string]*SessionInfo),
	}
	
	sm.loadSessions()
	sm.buildUI()
	sm.setupKeyboardCapture()
	return sm
}

// loadSessions loads sessions from the YAML file
func (sm *SessionManager) loadSessions() {
	sessionPath := DefaultSessionPath()
	sm.sessionStore = NewSessionStore(sessionPath)
	
	if err := sm.sessionStore.Load(); err != nil {
		log.Printf("Warning: Could not load sessions: %v", err)
	}
	
	sm.savedSessions = sm.sessionStore.GetSessions()
	sm.filteredSessions = sm.savedSessions
	
	if len(sm.savedSessions) == 0 {
		log.Printf("No sessions found, adding examples")
		sm.savedSessions = []SessionInfo{
			{ID: "1", Name: "Example-Server", Host: "192.168.1.1", Port: 22, Username: "admin", AuthType: AuthPassword, Group: "Examples"},
		}
		sm.filteredSessions = sm.savedSessions
	}
	
	sm.rebuildTreeData()
	log.Printf("Loaded %d sessions", len(sm.savedSessions))
}

// rebuildTreeData builds the tree structure from filtered sessions
func (sm *SessionManager) rebuildTreeData() {
	sm.treeData = make(map[string][]string)
	sm.sessionByID = make(map[string]*SessionInfo)
	
	// Group sessions by folder
	folders := make(map[string][]*SessionInfo)
	for i := range sm.filteredSessions {
		session := &sm.filteredSessions[i]
		group := session.Group
		if group == "" {
			group = "Default"
		}
		folders[group] = append(folders[group], session)
	}
	
	// Build root level (folders)
	var folderNames []string
	for name := range folders {
		folderNames = append(folderNames, name)
	}
	sort.Strings(folderNames)
	
	// Root children are folder IDs (prefixed to distinguish from sessions)
	var rootChildren []string
	for _, name := range folderNames {
		folderID := "folder:" + name
		rootChildren = append(rootChildren, folderID)
		
		// Build folder children (session IDs)
		var sessionIDs []string
		for _, session := range folders[name] {
			sessionID := "session:" + session.ID
			sessionIDs = append(sessionIDs, sessionID)
			sm.sessionByID[sessionID] = session
		}
		sm.treeData[folderID] = sessionIDs
	}
	sm.treeData[""] = rootChildren
}

// saveSessions saves sessions to the YAML file
func (sm *SessionManager) saveSessions() {
	if sm.sessionStore == nil {
		return
	}
	if err := sm.sessionStore.Save(); err != nil {
		log.Printf("Error saving sessions: %v", err)
	}
}

// refreshSessions reloads sessions from store and applies current filter
func (sm *SessionManager) refreshSessions() {
	sm.savedSessions = sm.sessionStore.GetSessions()
	sm.applyFilter()
}

// applyFilter filters sessions based on current filterText
func (sm *SessionManager) applyFilter() {
	if sm.filterText == "" {
		sm.filteredSessions = sm.savedSessions
	} else {
		query := strings.ToLower(sm.filterText)
		sm.filteredSessions = nil
		
		for _, session := range sm.savedSessions {
			searchText := strings.ToLower(fmt.Sprintf("%s %s %s %s %s %s",
				session.Name,
				session.Host,
				session.Group,
				session.DeviceType,
				session.Vendor,
				session.Username,
			))
			
			if strings.Contains(searchText, query) {
				sm.filteredSessions = append(sm.filteredSessions, session)
			}
		}
	}
	
	// Clear selection if selected item is no longer visible
	if sm.selectedSession != nil {
		found := false
		for _, s := range sm.filteredSessions {
			if s.ID == sm.selectedSession.ID {
				found = true
				break
			}
		}
		if !found {
			sm.selectedSession = nil
			sm.selectedNodeID = ""
		}
	}
	
	sm.rebuildTreeData()
	
	if sm.sessionTree != nil {
		sm.sessionTree.Refresh()
		// Expand all folders when filtering
		if sm.filterText != "" {
			for nodeID := range sm.treeData {
				if strings.HasPrefix(nodeID, "folder:") {
					sm.sessionTree.OpenBranch(nodeID)
				}
			}
		}
	}
}

// setupKeyboardCapture sets up window-level keyboard event forwarding
func (sm *SessionManager) setupKeyboardCapture() {
	sm.window.Canvas().SetOnTypedKey(func(key *fyne.KeyEvent) {
		// Don't forward if search box is focused
		if sm.window.Canvas().Focused() == sm.searchEntry {
			return
		}
		
		terminal := sm.getActiveTerminal()
		if terminal == nil {
			return
		}
		
		if sm.window.Canvas().Focused() != terminal {
			sm.window.Canvas().Focus(terminal)
		}
		
		terminal.TypedKey(key)
	})
	
	sm.window.Canvas().SetOnTypedRune(func(r rune) {
		if sm.window.Canvas().Focused() == sm.searchEntry {
			return
		}
		
		terminal := sm.getActiveTerminal()
		if terminal == nil {
			return
		}
		
		if sm.window.Canvas().Focused() != terminal {
			sm.window.Canvas().Focus(terminal)
		}
		
		terminal.TypedRune(r)
	})
}

// getActiveTerminal returns the terminal widget in the currently selected tab
func (sm *SessionManager) getActiveTerminal() *SSHTerminalWidget {
	selected := sm.tabContainer.Selected()
	if selected == nil {
		return nil
	}
	
	sm.tabsMutex.RLock()
	defer sm.tabsMutex.RUnlock()
	
	for _, sessionTab := range sm.activeTabs {
		if sessionTab.Tab == selected {
			return sessionTab.Terminal
		}
	}
	
	return nil
}

// buildUI constructs the session manager interface
func (sm *SessionManager) buildUI() {
	// Build session tree (left sidebar)
	sm.sessionTree = sm.buildSessionTree()
	
	// Build tab container (main area)
	sm.tabContainer = container.NewDocTabs()
	sm.tabContainer.CloseIntercept = sm.handleTabClose
	
	// Create sidebar with search
	sidebar := container.NewBorder(
		container.NewVBox(
			sm.buildSidebarHeader(),
			sm.buildSearchBox(),
		),
		sm.buildSidebarFooter(),
		nil, nil,
		container.NewVScroll(sm.sessionTree),
	)
	
	// Set sidebar width
	split := container.NewHSplit(sidebar, sm.tabContainer)
	split.SetOffset(0.2)
	
	sm.mainContainer = container.NewMax(split)
}

// buildSearchBox creates the search/filter entry
func (sm *SessionManager) buildSearchBox() fyne.CanvasObject {
	sm.searchEntry = widget.NewEntry()
	sm.searchEntry.SetPlaceHolder("🔍 Filter sessions...")
	
	sm.searchEntry.OnChanged = func(text string) {
		sm.filterText = text
		sm.applyFilter()
	}
	
	clearBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		sm.searchEntry.SetText("")
		sm.filterText = ""
		sm.applyFilter()
		sm.window.Canvas().Focus(sm.searchEntry)
	})
	clearBtn.Importance = widget.LowImportance
	
	return container.NewBorder(nil, nil, nil, clearBtn, sm.searchEntry)
}

// buildSessionTree creates the tree widget for sessions
func (sm *SessionManager) buildSessionTree() *widget.Tree {
	tree := widget.NewTree(
		// ChildUIDs - return children for a given node
		func(uid widget.TreeNodeID) []widget.TreeNodeID {
			return sm.treeData[uid]
		},
		
		// IsBranch - folders are branches, sessions are leaves
		func(uid widget.TreeNodeID) bool {
			return uid == "" || strings.HasPrefix(uid, "folder:")
		},
		
		// CreateNode - create template for branch or leaf
		func(branch bool) fyne.CanvasObject {
			if branch {
				// Folder row
				icon := widget.NewIcon(theme.FolderIcon())
				name := widget.NewLabel("Folder Name")
				name.TextStyle = fyne.TextStyle{Bold: true}
				count := widget.NewLabel("(0)")
				count.TextStyle = fyne.TextStyle{Italic: true}
				
				return container.NewHBox(icon, name, count)
			}
			
			// Session row
			icon := widget.NewIcon(theme.ComputerIcon())
			name := widget.NewLabel("Session Name")
			name.TextStyle = fyne.TextStyle{Bold: true}
			host := widget.NewLabel("host:port")
			status := widget.NewLabel("")
			
			return container.NewHBox(
				icon,
				container.NewVBox(name, host),
				status,
			)
		},
		
		// UpdateNode - populate with actual data
		func(uid widget.TreeNodeID, branch bool, o fyne.CanvasObject) {
			if branch {
				// Update folder
				box := o.(*fyne.Container)
				icon := box.Objects[0].(*widget.Icon)
				nameLabel := box.Objects[1].(*widget.Label)
				countLabel := box.Objects[2].(*widget.Label)
				
				if uid == "" {
					nameLabel.SetText("Sessions")
					countLabel.SetText("")
					icon.SetResource(theme.FolderIcon())
				} else {
					folderName := strings.TrimPrefix(uid, "folder:")
					nameLabel.SetText(folderName)
					
					// Count sessions in folder
					count := len(sm.treeData[uid])
					countLabel.SetText(fmt.Sprintf("(%d)", count))
					
					// Use open/closed folder icon based on state
					if sm.sessionTree != nil && sm.sessionTree.IsBranchOpen(uid) {
						icon.SetResource(theme.FolderOpenIcon())
					} else {
						icon.SetResource(theme.FolderIcon())
					}
				}
			} else {
				// Update session
				session := sm.sessionByID[uid]
				if session == nil {
					return
				}
				
				box := o.(*fyne.Container)
				vbox := box.Objects[1].(*fyne.Container)
				nameLabel := vbox.Objects[0].(*widget.Label)
				hostLabel := vbox.Objects[1].(*widget.Label)
				statusLabel := box.Objects[2].(*widget.Label)
				
				nameLabel.SetText(session.Name)
				hostLabel.SetText(fmt.Sprintf("%s:%d", session.Host, session.Port))
				
				// Show connection status
				sm.tabsMutex.RLock()
				statusText := ""
				for _, tab := range sm.activeTabs {
					if tab.Info.ID == session.ID {
						switch tab.State {
						case StateConnected:
							statusText = "●"
						case StateConnecting, StateAuthenticating:
							statusText = "○"
						case StateError:
							statusText = "✗"
						}
						break
					}
				}
				sm.tabsMutex.RUnlock()
				statusLabel.SetText(statusText)
			}
		},
	)
	
	// Handle selection
	tree.OnSelected = func(uid widget.TreeNodeID) {
		sm.selectedNodeID = uid
		
		if strings.HasPrefix(uid, "session:") {
			sm.selectedSession = sm.sessionByID[uid]
			if sm.selectedSession != nil {
				log.Printf("Selected session: %s", sm.selectedSession.Name)
			}
		} else {
			sm.selectedSession = nil
		}
	}
	
	// Handle branch open/close to update folder icons
	tree.OnBranchOpened = func(uid widget.TreeNodeID) {
		tree.Refresh()
	}
	tree.OnBranchClosed = func(uid widget.TreeNodeID) {
		tree.Refresh()
	}
	
	// Open all folders by default
	for nodeID := range sm.treeData {
		if strings.HasPrefix(nodeID, "folder:") {
			tree.OpenBranch(nodeID)
		}
	}
	
	return tree
}

// buildSidebarHeader creates the sidebar header with title and buttons
func (sm *SessionManager) buildSidebarHeader() fyne.CanvasObject {
	title := widget.NewLabel("Sessions")
	title.TextStyle = fyne.TextStyle{Bold: true}

	quickBtn := widget.NewButtonWithIcon("", theme.MediaFastForwardIcon(), func() {
		sm.showQuickConnectDialog()
	})
	quickBtn.Importance = widget.LowImportance

	// Changed to DocumentCreateIcon to free up SettingsIcon
	editBtn := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() {
		sm.showSessionEditor()
	})
	editBtn.Importance = widget.LowImportance

	addBtn := widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() {
		sm.showAddSessionDialog()
	})
	addBtn.Importance = widget.LowImportance

	// NEW: Settings button
	settingsBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		GetSettings().ShowSettingsDialog(sm.window)
	})
	settingsBtn.Importance = widget.LowImportance

	buttons := container.NewHBox(quickBtn, editBtn, addBtn, settingsBtn)

	return container.NewBorder(nil, nil, title, buttons)
}

// buildSidebarFooter creates the connect button
func (sm *SessionManager) buildSidebarFooter() fyne.CanvasObject {
	connectBtn := widget.NewButton("Connect", func() {
		if sm.selectedSession != nil {
			sm.connectToSession(*sm.selectedSession)
		}
	})
	connectBtn.Importance = widget.HighImportance
	
	return container.NewPadded(connectBtn)
}

// GetContainer returns the main container for embedding in your app
func (sm *SessionManager) GetContainer() *fyne.Container {
	return sm.mainContainer
}

// getDefaultKeyPath returns the default SSH key path from settings
func getDefaultKeyPath() string {
	settings := GetSettings().Get()
	if settings.DefaultKeyPath != "" {
		return settings.DefaultKeyPath
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "~/.ssh/id_rsa"
	}
	return filepath.Join(homeDir, ".ssh", "id_rsa")
}

// showQuickConnectDialog shows a dialog for quick ad-hoc connections
func (sm *SessionManager) showQuickConnectDialog() {
	hostEntry := widget.NewEntry()
	hostEntry.SetPlaceHolder("192.168.1.1 or hostname")
	
	portEntry := widget.NewEntry()
	portEntry.SetText("22")
	
	userEntry := widget.NewEntry()
	userEntry.SetPlaceHolder("admin")
	
	authSelect := widget.NewSelect([]string{"Password", "SSH Key"}, nil)
	authSelect.SetSelected("Password")
	
	passEntry := widget.NewPasswordEntry()
	passEntry.SetPlaceHolder("Enter password")
	
	keyPathEntry := widget.NewEntry()
	keyPathEntry.SetText(getDefaultKeyPath())
	keyPathEntry.Disable()
	
	keyPassEntry := widget.NewPasswordEntry()
	keyPassEntry.SetPlaceHolder("Key passphrase (if encrypted)")
	keyPassEntry.Disable()
	
	authSelect.OnChanged = func(selected string) {
		if selected == "SSH Key" {
			passEntry.Disable()
			keyPathEntry.Enable()
			keyPassEntry.Enable()
		} else {
			passEntry.Enable()
			keyPathEntry.Disable()
			keyPassEntry.Disable()
		}
	}
	
	items := []*widget.FormItem{
		widget.NewFormItem("Host", hostEntry),
		widget.NewFormItem("Port", portEntry),
		widget.NewFormItem("Username", userEntry),
		widget.NewFormItem("Auth Type", authSelect),
		widget.NewFormItem("Password", passEntry),
		widget.NewFormItem("Key Path", keyPathEntry),
		widget.NewFormItem("Key Passphrase", keyPassEntry),
	}
	
	d := dialog.NewForm("Quick Connect", "Connect", "Cancel", items,
		func(confirmed bool) {
			if !confirmed {
				return
			}
			
			if hostEntry.Text == "" {
				dialog.ShowError(fmt.Errorf("host is required"), sm.window)
				return
			}
			if userEntry.Text == "" {
				dialog.ShowError(fmt.Errorf("username is required"), sm.window)
				return
			}
			
			port := 22
			if portEntry.Text != "" {
				fmt.Sscanf(portEntry.Text, "%d", &port)
			}
			
			session := SessionInfo{
				ID:       uuid.New().String(),
				Name:     fmt.Sprintf("%s@%s", userEntry.Text, hostEntry.Text),
				Host:     hostEntry.Text,
				Port:     port,
				Username: userEntry.Text,
				Group:    "Quick Connect",
			}
			
			if authSelect.Selected == "SSH Key" {
				session.AuthType = AuthPublicKey
				session.KeyPath = keyPathEntry.Text
				session.KeyPassphrase = keyPassEntry.Text
				
				keyPath := session.KeyPath
				if len(keyPath) > 2 && keyPath[:2] == "~/" {
					homeDir, _ := os.UserHomeDir()
					keyPath = filepath.Join(homeDir, keyPath[2:])
				}
				if _, err := os.Stat(keyPath); os.IsNotExist(err) {
					dialog.ShowError(fmt.Errorf("key file not found: %s", session.KeyPath), sm.window)
					return
				}
				
				log.Printf("Quick Connect: Using SSH key auth with %s", session.KeyPath)
			} else {
				session.AuthType = AuthPassword
				session.Password = passEntry.Text
			}
			
			sm.connectToSession(session)
		},
		sm.window,
	)
	
	d.Resize(fyne.NewSize(450, 350))
	d.Show()
	sm.window.Canvas().Focus(hostEntry)
}

// showSessionEditor opens the full session editor modal
func (sm *SessionManager) showSessionEditor() {
	if sm.sessionStore == nil {
		dialog.ShowError(fmt.Errorf("session store not initialized"), sm.window)
		return
	}
	
	editor := NewSessionEditor(sm.window, sm.sessionStore, func() {
		sm.refreshSessions()
	})
	
	editor.Show()
}

// connectToSession creates a new terminal tab and connects
func (sm *SessionManager) connectToSession(session SessionInfo) {
	log.Printf("Connecting to %s (%s@%s:%d) via %s",
		session.Name, session.Username, session.Host, session.Port, session.AuthType)
	
	if session.Username == "" {
		sm.promptCredentialsAndConnect(session)
		return
	}
	
	if session.AuthType == AuthPublicKey {
		sm.doConnect(session, "")
		return
	}
	
	if session.Password == "" && (session.AuthType == AuthPassword || session.AuthType == AuthKeyboardInteractive) {
		sm.promptPasswordAndConnect(session)
		return
	}
	
	sm.doConnect(session, session.Password)
}

// promptCredentialsAndConnect shows a dialog for both username and password
func (sm *SessionManager) promptCredentialsAndConnect(session SessionInfo) {
	userEntry := widget.NewEntry()
	userEntry.SetPlaceHolder("username")
	
	passEntry := widget.NewPasswordEntry()
	passEntry.SetPlaceHolder("password")
	
	items := []*widget.FormItem{
		widget.NewFormItem("Username", userEntry),
		widget.NewFormItem("Password", passEntry),
	}
	
	d := dialog.NewForm(
		fmt.Sprintf("Connect to %s", session.Host),
		"Connect", "Cancel",
		items,
		func(confirmed bool) {
			if confirmed && userEntry.Text != "" {
				session.Username = userEntry.Text
				sm.doConnect(session, passEntry.Text)
			}
		},
		sm.window,
	)
	d.Resize(fyne.NewSize(350, 200))
	d.Show()
	sm.window.Canvas().Focus(userEntry)
}

// promptPasswordAndConnect shows a password dialog then connects
func (sm *SessionManager) promptPasswordAndConnect(session SessionInfo) {
	entry := widget.NewPasswordEntry()
	entry.SetPlaceHolder("Enter password")
	
	items := []*widget.FormItem{
		widget.NewFormItem("Password", entry),
	}
	
	d := dialog.NewForm(
		fmt.Sprintf("Connect to %s@%s", session.Username, session.Host),
		"Connect", "Cancel",
		items,
		func(confirmed bool) {
			if confirmed && entry.Text != "" {
				sm.doConnect(session, entry.Text)
			} else if confirmed {
				dialog.ShowError(fmt.Errorf("password is required"), sm.window)
			}
		},
		sm.window,
	)
	d.Resize(fyne.NewSize(400, 150))
	d.Show()
	sm.window.Canvas().Focus(entry)
}

// doConnect performs the actual SSH connection
func (sm *SessionManager) doConnect(session SessionInfo, password string) {
	tabID := uuid.New().String()
	
	terminal := NewSSHTerminalWidget(true)
	
	sshConfig := DefaultSSHConfig()
	sshConfig.Host = session.Host
	sshConfig.Port = session.Port
	sshConfig.Username = session.Username
	sshConfig.Password = password
	
	if terminal.cols > 0 && terminal.rows > 0 {
		sshConfig.Cols = terminal.cols
		sshConfig.Rows = terminal.rows
	}
	
	switch session.AuthType {
	case AuthPublicKey:
		sshConfig.PrivateKeyPath = session.KeyPath
		sshConfig.KeyPassphrase = session.KeyPassphrase
		sshConfig.UseAgent = false
		log.Printf("Configured SSH key auth: %s", session.KeyPath)
	case AuthPassword:
		sshConfig.UseAgent = false
	}
	
	terminal.SetSSHConfig(sshConfig)
	
	terminal.SetAuthUIHandler(func(prompt string, echo bool) (string, error) {
		return sm.showAuthPrompt(prompt, echo)
	})
	
	tabName := session.Name
	sm.tabsMutex.RLock()
	duplicateCount := 0
	for _, tab := range sm.activeTabs {
		if tab.Info.Host == session.Host && tab.Info.Port == session.Port {
			duplicateCount++
		}
	}
	sm.tabsMutex.RUnlock()
	if duplicateCount > 0 {
		tabName = fmt.Sprintf("%s (%d)", session.Name, duplicateCount+1)
	}
	
	tabItem := container.NewTabItem(tabName, terminal)
	
	sessionTab := &SessionTab{
		TabID:    tabID,
		Info:     session,
		Terminal: terminal,
		Tab:      tabItem,
		State:    StateDisconnected,
	}
	
	terminal.SetStateChangeHandler(func(state ConnectionState) {
		sessionTab.State = state
		sm.sessionTree.Refresh() // Update status indicators in tree
		
		switch state {
		case StateConnecting:
			tabItem.Text = fmt.Sprintf("%s (connecting...)", tabName)
		case StateAuthenticating:
			tabItem.Text = fmt.Sprintf("%s (authenticating...)", tabName)
		case StateConnected:
			tabItem.Text = tabName
			sm.window.Canvas().Focus(terminal)
		case StateError:
			tabItem.Text = fmt.Sprintf("%s (error)", tabName)
		case StateDisconnected:
			tabItem.Text = fmt.Sprintf("%s (disconnected)", tabName)
		}
		sm.tabContainer.Refresh()
	})
	
	terminal.SetErrorHandler(func(err error) {
		log.Printf("SSH error for %s [%s]: %v", session.Name, tabID, err)
		dialog.ShowError(err, sm.window)
	})
	
	sm.tabsMutex.Lock()
	sm.activeTabs[tabID] = sessionTab
	sm.tabsMutex.Unlock()
	
	sm.tabContainer.Append(tabItem)
	sm.tabContainer.Select(tabItem)
	
	go func() {
		if err := terminal.ConnectSSH(); err != nil {
			log.Printf("Failed to connect to %s [%s]: %v", session.Name, tabID, err)
		}
	}()
}

// showAuthPrompt shows a dialog for authentication prompts
func (sm *SessionManager) showAuthPrompt(prompt string, echo bool) (string, error) {
	resultChan := make(chan string, 1)
	errChan := make(chan error, 1)
	
	fyne.Do(func() {
		var entry *widget.Entry
		if echo {
			entry = widget.NewEntry()
		} else {
			entry = widget.NewPasswordEntry()
		}
		
		items := []*widget.FormItem{
			widget.NewFormItem(prompt, entry),
		}
		
		d := dialog.NewForm("Authentication Required", "OK", "Cancel", items,
			func(confirmed bool) {
				if confirmed {
					resultChan <- entry.Text
				} else {
					errChan <- fmt.Errorf("authentication cancelled by user")
				}
			}, sm.window)
		d.Resize(fyne.NewSize(400, 150))
		d.Show()
		sm.window.Canvas().Focus(entry)
	})
	
	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errChan:
		return "", err
	}
}

// handleTabClose handles graceful tab closing
func (sm *SessionManager) handleTabClose(tab *container.TabItem) {
	sm.tabsMutex.Lock()
	var sessionTab *SessionTab
	var tabID string
	for id, st := range sm.activeTabs {
		if st.Tab == tab {
			sessionTab = st
			tabID = id
			break
		}
	}
	sm.tabsMutex.Unlock()

	if sessionTab == nil {
		sm.tabContainer.Remove(tab)
		return
	}

	if sessionTab.State != StateConnected {
		go func() {
			sessionTab.Terminal.Disconnect()
			fyne.Do(func() {
				sm.tabsMutex.Lock()
				delete(sm.activeTabs, tabID)
				sm.tabsMutex.Unlock()
				sm.tabContainer.Remove(tab)
				sm.sessionTree.Refresh()
			})
		}()
		return
	}

	dialog.ShowConfirm(
		"Close Session",
		fmt.Sprintf("Close session '%s'?\n\nHost: %s@%s",
			sessionTab.Info.Name, sessionTab.Info.Username, sessionTab.Info.Host),
		func(confirmed bool) {
			if !confirmed {
				return
			}

			tab.Text = fmt.Sprintf("%s (closing...)", sessionTab.Info.Name)
			sm.tabContainer.Refresh()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			go func() {
				sessionTab.Terminal.DisconnectWithContext(ctx)

				fyne.Do(func() {
					sm.tabsMutex.Lock()
					delete(sm.activeTabs, tabID)
					sm.tabsMutex.Unlock()

					sm.tabContainer.Remove(tab)
					sm.sessionTree.Refresh()
				})
			}()
		},
		sm.window,
	)
}

// showAddSessionDialog shows the dialog for adding a new session
func (sm *SessionManager) showAddSessionDialog() {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("My Server")
	
	hostEntry := widget.NewEntry()
	hostEntry.SetPlaceHolder("192.168.1.1")
	
	portEntry := widget.NewEntry()
	portEntry.SetText("22")
	
	userEntry := widget.NewEntry()
	userEntry.SetPlaceHolder("admin")
	
	authSelect := widget.NewSelect([]string{"Password", "SSH Key", "Keyboard Interactive"}, nil)
	authSelect.SetSelected("Password")
	
	keyPathEntry := widget.NewEntry()
	keyPathEntry.SetText(getDefaultKeyPath())
	keyPathEntry.Disable()
	
	keyPassEntry := widget.NewPasswordEntry()
	keyPassEntry.SetPlaceHolder("Key passphrase (if encrypted)")
	keyPassEntry.Disable()
	
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Leave blank to prompt on connect")
	
	authSelect.OnChanged = func(s string) {
		if s == "SSH Key" {
			keyPathEntry.Enable()
			keyPassEntry.Enable()
			passwordEntry.Disable()
		} else {
			keyPathEntry.Disable()
			keyPassEntry.Disable()
			passwordEntry.Enable()
		}
	}
	
	groupEntry := widget.NewEntry()
	groupEntry.SetPlaceHolder("Servers")
	
	items := []*widget.FormItem{
		widget.NewFormItem("Name", nameEntry),
		widget.NewFormItem("Host", hostEntry),
		widget.NewFormItem("Port", portEntry),
		widget.NewFormItem("Username", userEntry),
		widget.NewFormItem("Auth Type", authSelect),
		widget.NewFormItem("Password", passwordEntry),
		widget.NewFormItem("Key Path", keyPathEntry),
		widget.NewFormItem("Key Passphrase", keyPassEntry),
		widget.NewFormItem("Group", groupEntry),
	}
	
	d := dialog.NewForm("Add Session", "Add", "Cancel", items,
		func(confirmed bool) {
			if confirmed && nameEntry.Text != "" && hostEntry.Text != "" {
				var authType AuthMethod
				switch authSelect.Selected {
				case "Password":
					authType = AuthPassword
				case "SSH Key":
					authType = AuthPublicKey
				case "Keyboard Interactive":
					authType = AuthKeyboardInteractive
				}
				
				port := 22
				fmt.Sscanf(portEntry.Text, "%d", &port)
				
				newSession := SessionInfo{
					ID:            fmt.Sprintf("%d", len(sm.savedSessions)+1),
					Name:          nameEntry.Text,
					Host:          hostEntry.Text,
					Port:          port,
					Username:      userEntry.Text,
					Password:      passwordEntry.Text,
					AuthType:      authType,
					KeyPath:       keyPathEntry.Text,
					KeyPassphrase: keyPassEntry.Text,
					Group:         groupEntry.Text,
				}
				
				sm.savedSessions = append(sm.savedSessions, newSession)
				sm.applyFilter()
				
				if sm.sessionStore != nil {
					groupName := groupEntry.Text
					if groupName == "" {
						groupName = "Default"
					}
					sm.sessionStore.AddSession(groupName, newSession)
					sm.saveSessions()
				}
				
				log.Printf("Added new session: %s", newSession.Name)
			}
		}, sm.window)
	
	d.Resize(fyne.NewSize(450, 450))
	d.Show()
}

// DisconnectAll closes all active sessions
func (sm *SessionManager) DisconnectAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	sm.tabsMutex.Lock()
	tabs := make([]*SessionTab, 0, len(sm.activeTabs))
	for _, tab := range sm.activeTabs {
		tabs = append(tabs, tab)
	}
	sm.activeTabs = make(map[string]*SessionTab)
	sm.tabsMutex.Unlock()

	var wg sync.WaitGroup
	for _, tab := range tabs {
		wg.Add(1)
		go func(t *SessionTab) {
			defer wg.Done()
			t.Terminal.DisconnectWithContext(ctx)
		}(tab)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("All sessions cleanly disconnected")
	case <-ctx.Done():
		log.Printf("Disconnect timeout - some sessions may linger")
	}

	sm.sessionTree.Refresh()
}