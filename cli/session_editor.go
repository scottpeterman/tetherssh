// session_editor.go - Modal dialog for CRUD operations on sessions
// Provides a full editor interface for managing sessions and folders
package main

import (
	"fmt"
	"log"
	"sort"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// SessionEditor provides a modal interface for managing sessions
type SessionEditor struct {
	window       fyne.Window
	sessionStore *SessionStore
	onSave       func() // Callback when sessions are modified

	// UI components
	folderList   *widget.List
	sessionList  *widget.List
	
	// Current selection state
	selectedFolder  string
	selectedSession *SessionInfo
	folders         []string
	sessions        []SessionInfo
}

// NewSessionEditor creates a new session editor
func NewSessionEditor(window fyne.Window, store *SessionStore, onSave func()) *SessionEditor {
	editor := &SessionEditor{
		window:       window,
		sessionStore: store,
		onSave:       onSave,
	}
	editor.refreshData()
	return editor
}

// refreshData reloads folders and sessions from the store
func (e *SessionEditor) refreshData() {
	e.folders = e.sessionStore.GetFolders()
	sort.Strings(e.folders)
	
	// If we have a selected folder, load its sessions
	if e.selectedFolder != "" {
		byFolder := e.sessionStore.GetSessionsByFolder()
		e.sessions = byFolder[e.selectedFolder]
	} else {
		e.sessions = nil
	}
}

// Show displays the session editor modal
func (e *SessionEditor) Show() {
	e.refreshData()
	
	// Build the editor UI
	content := e.buildUI()
	
	// Create a custom dialog with the editor content
	d := dialog.NewCustom("Session Manager", "Close", content, e.window)
	d.Resize(fyne.NewSize(800, 500))
	d.Show()
}

// buildUI constructs the editor interface
func (e *SessionEditor) buildUI() fyne.CanvasObject {
	// Left panel: Folders
	folderHeader := container.NewBorder(
		nil, nil,
		widget.NewLabelWithStyle("Folders", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(
			widget.NewButtonWithIcon("", theme.ContentAddIcon(), e.showAddFolderDialog),
			widget.NewButtonWithIcon("", theme.DeleteIcon(), e.deleteSelectedFolder),
		),
	)
	
	e.folderList = widget.NewList(
		func() int { return len(e.folders) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewIcon(theme.FolderIcon()),
				widget.NewLabel("Folder Name"),
			)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			box := item.(*fyne.Container)
			label := box.Objects[1].(*widget.Label)
			label.SetText(e.folders[id])
		},
	)
	
	e.folderList.OnSelected = func(id widget.ListItemID) {
		e.selectedFolder = e.folders[id]
		e.selectedSession = nil
		e.refreshData()
		if e.sessionList != nil {
			e.sessionList.UnselectAll()
			e.sessionList.Refresh()
		}
	}
	
	folderPanel := container.NewBorder(folderHeader, nil, nil, nil,
		container.NewVScroll(e.folderList))
	
	// Right panel: Sessions
	sessionHeader := container.NewBorder(
		nil, nil,
		widget.NewLabelWithStyle("Sessions", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(
			widget.NewButtonWithIcon("", theme.ContentAddIcon(), e.showAddSessionDialog),
			widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), e.showEditSessionDialog),
			widget.NewButtonWithIcon("", theme.DeleteIcon(), e.deleteSelectedSession),
		),
	)
	
	e.sessionList = widget.NewList(
		func() int { return len(e.sessions) },
		func() fyne.CanvasObject {
			nameLabel := widget.NewLabel("Session Name")
			nameLabel.TextStyle = fyne.TextStyle{Bold: true}
			hostLabel := widget.NewLabel("host:port")
			typeLabel := widget.NewLabel("")
			
			return container.NewHBox(
				widget.NewIcon(theme.ComputerIcon()),
				container.NewVBox(nameLabel, hostLabel),
				typeLabel,
			)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			if id >= len(e.sessions) {
				return
			}
			session := e.sessions[id]
			box := item.(*fyne.Container)
			vbox := box.Objects[1].(*fyne.Container)
			nameLabel := vbox.Objects[0].(*widget.Label)
			hostLabel := vbox.Objects[1].(*widget.Label)
			typeLabel := box.Objects[2].(*widget.Label)
			
			nameLabel.SetText(session.Name)
			hostLabel.SetText(fmt.Sprintf("%s:%d", session.Host, session.Port))
			
			// Show device type if available
			if session.DeviceType != "" {
				typeLabel.SetText(session.DeviceType)
			} else if session.Vendor != "" {
				typeLabel.SetText(session.Vendor)
			} else {
				typeLabel.SetText("")
			}
		},
	)
	
	e.sessionList.OnSelected = func(id widget.ListItemID) {
		if id < len(e.sessions) {
			e.selectedSession = &e.sessions[id]
		}
	}
	
	// Double-click to edit
	// Note: Fyne doesn't have native double-click on lists, 
	// so we use the edit button instead
	
	sessionPanel := container.NewBorder(sessionHeader, nil, nil, nil,
		container.NewVScroll(e.sessionList))
	
	// Split view
	split := container.NewHSplit(folderPanel, sessionPanel)
	split.SetOffset(0.3) // 30% for folders
	
	// Bottom toolbar
	toolbar := container.NewHBox(
		widget.NewButtonWithIcon("Import YAML", theme.FolderOpenIcon(), e.showImportDialog),
		widget.NewButtonWithIcon("Export YAML", theme.DocumentSaveIcon(), e.exportSessions),
		widget.NewLabel(fmt.Sprintf("Path: %s", e.sessionStore.filePath)),
	)
	
	return container.NewBorder(nil, toolbar, nil, nil, split)
}

// showAddFolderDialog shows dialog to add a new folder
func (e *SessionEditor) showAddFolderDialog() {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Folder name")
	
	items := []*widget.FormItem{
		widget.NewFormItem("Name", nameEntry),
	}
	
	d := dialog.NewForm("Add Folder", "Add", "Cancel", items,
		func(confirmed bool) {
			if confirmed && nameEntry.Text != "" {
				e.sessionStore.AddFolder(nameEntry.Text)
				e.saveAndRefresh()
			}
		}, e.window)
	d.Resize(fyne.NewSize(300, 150))
	d.Show()
}

// deleteSelectedFolder deletes the currently selected folder
func (e *SessionEditor) deleteSelectedFolder() {
	if e.selectedFolder == "" {
		dialog.ShowInformation("No Selection", "Please select a folder to delete.", e.window)
		return
	}
	
	// Check if folder has sessions
	byFolder := e.sessionStore.GetSessionsByFolder()
	sessions := byFolder[e.selectedFolder]
	
	message := fmt.Sprintf("Delete folder '%s'?", e.selectedFolder)
	if len(sessions) > 0 {
		message = fmt.Sprintf("Delete folder '%s' and its %d sessions?", e.selectedFolder, len(sessions))
	}
	
	dialog.ShowConfirm("Delete Folder", message,
		func(confirmed bool) {
			if confirmed {
				e.sessionStore.RemoveFolder(e.selectedFolder)
				e.selectedFolder = ""
				e.saveAndRefresh()
			}
		}, e.window)
}

// showAddSessionDialog shows dialog to add a new session
func (e *SessionEditor) showAddSessionDialog() {
	if e.selectedFolder == "" {
		dialog.ShowInformation("No Folder", "Please select a folder first.", e.window)
		return
	}
	
	e.showSessionFormDialog("Add Session", SessionInfo{}, func(session SessionInfo) {
		e.sessionStore.AddSession(e.selectedFolder, session)
		e.saveAndRefresh()
	})
}

// showEditSessionDialog shows dialog to edit the selected session
func (e *SessionEditor) showEditSessionDialog() {
	if e.selectedSession == nil {
		dialog.ShowInformation("No Selection", "Please select a session to edit.", e.window)
		return
	}
	
	e.showSessionFormDialog("Edit Session", *e.selectedSession, func(session SessionInfo) {
		// Remove old and add updated
		e.sessionStore.RemoveSession(e.selectedSession.ID)
		e.sessionStore.AddSession(e.selectedFolder, session)
		e.saveAndRefresh()
	})
}

// showSessionFormDialog shows the session edit form
func (e *SessionEditor) showSessionFormDialog(title string, session SessionInfo, onSave func(SessionInfo)) {
	// Basic fields
	nameEntry := widget.NewEntry()
	nameEntry.SetText(session.Name)
	nameEntry.SetPlaceHolder("Display name")
	
	hostEntry := widget.NewEntry()
	hostEntry.SetText(session.Host)
	hostEntry.SetPlaceHolder("192.168.1.1 or hostname")
	
	portEntry := widget.NewEntry()
	if session.Port > 0 {
		portEntry.SetText(strconv.Itoa(session.Port))
	} else {
		portEntry.SetText("22")
	}
	
	usernameEntry := widget.NewEntry()
	usernameEntry.SetText(session.Username)
	usernameEntry.SetPlaceHolder("Leave blank to prompt")
	
	// Auth type
	authSelect := widget.NewSelect([]string{"Password", "SSH Key", "Keyboard Interactive"}, nil)
	switch session.AuthType {
	case AuthPublicKey:
		authSelect.SetSelected("SSH Key")
	case AuthKeyboardInteractive:
		authSelect.SetSelected("Keyboard Interactive")
	default:
		authSelect.SetSelected("Password")
	}
	
	keyPathEntry := widget.NewEntry()
	keyPathEntry.SetText(session.KeyPath)
	if session.KeyPath == "" {
		keyPathEntry.SetText(getDefaultKeyPath())
	}
	keyPathEntry.Disable()
	
	// Device info fields
	deviceTypeEntry := widget.NewEntry()
	deviceTypeEntry.SetText(session.DeviceType)
	deviceTypeEntry.SetPlaceHolder("linux, cisco_ios, arista_eos, etc.")
	
	vendorEntry := widget.NewEntry()
	vendorEntry.SetText(session.Vendor)
	vendorEntry.SetPlaceHolder("Cisco, Arista, etc.")
	
	modelEntry := widget.NewEntry()
	modelEntry.SetText(session.Model)
	modelEntry.SetPlaceHolder("Model name")
	
	credsIDEntry := widget.NewEntry()
	credsIDEntry.SetText(session.CredsID)
	credsIDEntry.SetPlaceHolder("Credentials reference")
	
	// Toggle key path based on auth type
	authSelect.OnChanged = func(s string) {
		if s == "SSH Key" {
			keyPathEntry.Enable()
		} else {
			keyPathEntry.Disable()
		}
	}
	// Apply initial state
	if authSelect.Selected == "SSH Key" {
		keyPathEntry.Enable()
	}
	
	items := []*widget.FormItem{
		widget.NewFormItem("Display Name", nameEntry),
		widget.NewFormItem("Host", hostEntry),
		widget.NewFormItem("Port", portEntry),
		widget.NewFormItem("Username", usernameEntry),
		widget.NewFormItem("Auth Type", authSelect),
		widget.NewFormItem("Key Path", keyPathEntry),
		widget.NewFormItem("", widget.NewSeparator()),
		widget.NewFormItem("Device Type", deviceTypeEntry),
		widget.NewFormItem("Vendor", vendorEntry),
		widget.NewFormItem("Model", modelEntry),
		widget.NewFormItem("Creds ID", credsIDEntry),
	}
	
	d := dialog.NewForm(title, "Save", "Cancel", items,
		func(confirmed bool) {
			if !confirmed {
				return
			}
			
			// Validate
			if hostEntry.Text == "" {
				dialog.ShowError(fmt.Errorf("host is required"), e.window)
				return
			}
			
			// Parse port
			port := 22
			if portEntry.Text != "" {
				if p, err := strconv.Atoi(portEntry.Text); err == nil {
					port = p
				}
			}
			
			// Parse auth type
			var authType AuthMethod
			switch authSelect.Selected {
			case "SSH Key":
				authType = AuthPublicKey
			case "Keyboard Interactive":
				authType = AuthKeyboardInteractive
			default:
				authType = AuthPassword
			}
			
			// Build session
			newSession := SessionInfo{
				Name:       nameEntry.Text,
				Host:       hostEntry.Text,
				Port:       port,
				Username:   usernameEntry.Text,
				AuthType:   authType,
				KeyPath:    keyPathEntry.Text,
				DeviceType: deviceTypeEntry.Text,
				Vendor:     vendorEntry.Text,
				Model:      modelEntry.Text,
				CredsID:    credsIDEntry.Text,
				Group:      e.selectedFolder,
			}
			
			// Default display name to user@host if not provided
			if newSession.Name == "" {
				if newSession.Username != "" {
					newSession.Name = fmt.Sprintf("%s@%s", newSession.Username, newSession.Host)
				} else {
					newSession.Name = newSession.Host
				}
			}
			
			onSave(newSession)
		}, e.window)
	
	d.Resize(fyne.NewSize(450, 500))
	d.Show()
}

// deleteSelectedSession deletes the currently selected session
func (e *SessionEditor) deleteSelectedSession() {
	if e.selectedSession == nil {
		dialog.ShowInformation("No Selection", "Please select a session to delete.", e.window)
		return
	}
	
	dialog.ShowConfirm("Delete Session",
		fmt.Sprintf("Delete session '%s'?\n\nHost: %s:%d", 
			e.selectedSession.Name, e.selectedSession.Host, e.selectedSession.Port),
		func(confirmed bool) {
			if confirmed {
				e.sessionStore.RemoveSession(e.selectedSession.ID)
				e.selectedSession = nil
				e.saveAndRefresh()
			}
		}, e.window)
}

// showImportDialog shows a file picker to import sessions from YAML
func (e *SessionEditor) showImportDialog() {
	// For now, show a simple path entry dialog
	// Full file picker requires more Fyne setup
	pathEntry := widget.NewEntry()
	pathEntry.SetPlaceHolder("/path/to/sessions.yaml")
	
	items := []*widget.FormItem{
		widget.NewFormItem("YAML File", pathEntry),
	}
	
	d := dialog.NewForm("Import Sessions", "Import", "Cancel", items,
		func(confirmed bool) {
			if !confirmed || pathEntry.Text == "" {
				return
			}
			
			// Create temp store to load from file
			tempStore := NewSessionStore(pathEntry.Text)
			if err := tempStore.Load(); err != nil {
				dialog.ShowError(fmt.Errorf("failed to load: %w", err), e.window)
				return
			}
			
			// Merge folders and sessions
			imported := 0
			for _, folder := range tempStore.folders {
				e.sessionStore.AddFolder(folder.FolderName)
				for _, sess := range folder.Sessions {
					port := 22
					if sess.Port != "" {
						if p, err := strconv.Atoi(sess.Port); err == nil {
							port = p
						}
					}
					
					e.sessionStore.AddSession(folder.FolderName, SessionInfo{
						Name:       sess.DisplayName,
						Host:       sess.Host,
						Port:       port,
						DeviceType: sess.DeviceType,
						Vendor:     sess.Vendor,
						Model:      sess.Model,
						CredsID:    sess.CredsID,
					})
					imported++
				}
			}
			
			e.saveAndRefresh()
			dialog.ShowInformation("Import Complete", 
				fmt.Sprintf("Imported %d sessions from %d folders.", imported, len(tempStore.folders)),
				e.window)
		}, e.window)
	
	d.Resize(fyne.NewSize(450, 150))
	d.Show()
}

// exportSessions saves current sessions and shows confirmation
func (e *SessionEditor) exportSessions() {
	if err := e.sessionStore.Save(); err != nil {
		dialog.ShowError(err, e.window)
		return
	}
	
	dialog.ShowInformation("Export Complete", 
		fmt.Sprintf("Sessions saved to:\n%s", e.sessionStore.filePath),
		e.window)
}

// saveAndRefresh saves changes and refreshes the UI
func (e *SessionEditor) saveAndRefresh() {
	if err := e.sessionStore.Save(); err != nil {
		log.Printf("Error saving sessions: %v", err)
		dialog.ShowError(err, e.window)
		return
	}
	
	e.refreshData()
	
	if e.folderList != nil {
		e.folderList.Refresh()
	}
	if e.sessionList != nil {
		e.sessionList.Refresh()
	}
	
	// Notify parent to refresh its session list
	if e.onSave != nil {
		e.onSave()
	}
}

// UpdateSession is a helper to update a session (for external use)
func (s *SessionStore) UpdateSession(sessionID string, updated SessionInfo) bool {
	// Find the session
	for fi := range s.folders {
		for si := range s.folders[fi].Sessions {
			id := fmt.Sprintf("%s-%d", s.folders[fi].FolderName, si)
			if id == sessionID {
				// Update in place
				s.folders[fi].Sessions[si] = SessionYAML{
					DisplayName:     updated.Name,
					Host:            updated.Host,
					Port:            strconv.Itoa(updated.Port),
					DeviceType:      updated.DeviceType,
					Vendor:          updated.Vendor,
					Model:           updated.Model,
					CredsID:         updated.CredsID,
					SerialNumber:    s.folders[fi].Sessions[si].SerialNumber,
					SoftwareVersion: s.folders[fi].Sessions[si].SoftwareVersion,
				}
				return true
			}
		}
	}
	return false
}

// MoveSession moves a session from one folder to another
func (s *SessionStore) MoveSession(sessionID string, targetFolder string) bool {
	// Find the session
	for fi := range s.folders {
		for si := range s.folders[fi].Sessions {
			id := fmt.Sprintf("%s-%d", s.folders[fi].FolderName, si)
			if id == sessionID {
				// Copy session data
				sess := s.folders[fi].Sessions[si]
				
				// Remove from current folder
				s.folders[fi].Sessions = append(
					s.folders[fi].Sessions[:si],
					s.folders[fi].Sessions[si+1:]...,
				)
				
				// Find or create target folder
				var target *SessionFolder
				for i := range s.folders {
					if s.folders[i].FolderName == targetFolder {
						target = &s.folders[i]
						break
					}
				}
				
				if target == nil {
					s.folders = append(s.folders, SessionFolder{
						FolderName: targetFolder,
						Sessions:   []SessionYAML{},
					})
					target = &s.folders[len(s.folders)-1]
				}
				
				// Add to target folder
				target.Sessions = append(target.Sessions, sess)
				return true
			}
		}
	}
	return false
}