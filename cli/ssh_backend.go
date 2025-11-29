// ssh_backend.go - SSH connection backend for tetherssh
// Provides a unified interface for SSH connections with comprehensive auth support
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"tetherssh/internal/gopyte"
	"time"

	"fyne.io/fyne/v2"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// TerminalBackend defines the interface for terminal I/O backends
// This abstraction allows swapping between local PTY and SSH sessions
type TerminalBackend interface {
	Read(p []byte) (n int, err error)
	Write(p []byte) (n int, err error)
	Resize(cols, rows int) error
	Close() error
	IsConnected() bool
}

// ConnectionState represents the current state of an SSH connection
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateAuthenticating
	StateConnected
	StateError
	StateReconnecting
)

func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "Disconnected"
	case StateConnecting:
		return "Connecting"
	case StateAuthenticating:
		return "Authenticating"
	case StateConnected:
		return "Connected"
	case StateError:
		return "Error"
	case StateReconnecting:
		return "Reconnecting"
	default:
		return "Unknown"
	}
}

// AuthMethod represents the type of authentication being used
type AuthMethod int

const (
	AuthNone AuthMethod = iota
	AuthPassword
	AuthPublicKey
	AuthKeyboardInteractive
	AuthAgent
)

func (a AuthMethod) String() string {
	switch a {
	case AuthPassword:
		return "Password"
	case AuthPublicKey:
		return "Public Key"
	case AuthKeyboardInteractive:
		return "Keyboard Interactive"
	case AuthAgent:
		return "SSH Agent"
	default:
		return "None"
	}
}

// SSHConfig holds all configuration for an SSH connection
type SSHConfig struct {
	// Connection settings
	Host    string
	Port    int
	Timeout time.Duration

	// Authentication
	Username       string
	Password       string // Optional - for password auth
	PrivateKeyPath string // Optional - path to key file
	PrivateKey     []byte // Optional - in-memory key (takes precedence)
	KeyPassphrase  string // Optional - passphrase for encrypted keys
	UseAgent       bool   // Try SSH agent first

	// Host key verification
	HostKeyCallback   ssh.HostKeyCallback // Custom callback
	KnownHostsPath    string              // Path to known_hosts file
	InsecureIgnoreKey bool                // DANGER: Skip host key verification

	// Terminal settings
	TermType string // e.g., "xterm-256color"
	Cols     int
	Rows     int

	// Behavior
	KeepAliveInterval time.Duration // 0 = disabled
	KeepAliveMaxCount int           // Max missed keepalives before disconnect
}

// DefaultSSHConfig returns a config with sensible defaults
func DefaultSSHConfig() SSHConfig {
	homeDir, _ := os.UserHomeDir()
	return SSHConfig{
		Port:              22,
		Timeout:           30 * time.Second,
		TermType:          "xterm-256color",
		Cols:              80,
		Rows:              24,
		KnownHostsPath:    filepath.Join(homeDir, ".ssh", "known_hosts"),
		KeepAliveInterval: 30 * time.Second,
		KeepAliveMaxCount: 3,
		UseAgent:          true,
	}
}

// AuthPromptCallback is called when keyboard-interactive auth needs user input
// prompt: The question being asked (e.g., "Password:", "Verification code:")
// echo: Whether the input should be displayed (false for passwords)
// Returns the user's response or an error
type AuthPromptCallback func(prompt string, echo bool) (string, error)

// StateChangeCallback is called when connection state changes
type StateChangeCallback func(oldState, newState ConnectionState)

// SSHBackend implements TerminalBackend for SSH connections
type SSHBackend struct {
	config SSHConfig

	// SSH connection state
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	stdout  io.Reader
	stderr  io.Reader

	// Combined output reader
	outputReader *io.PipeReader
	outputWriter *io.PipeWriter

	// State management
	state      ConnectionState
	stateMutex sync.RWMutex
	lastError  error

	// Auth method that succeeded
	authMethod AuthMethod

	// Callbacks
	authPromptHandler  AuthPromptCallback
	stateChangeHandler StateChangeCallback

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Connection management
	connMutex     sync.Mutex
	keepAliveDone chan struct{}
}

// NewSSHBackend creates a new SSH backend with the given configuration
func NewSSHBackend(config SSHConfig) *SSHBackend {
	ctx, cancel := context.WithCancel(context.Background())

	// Apply defaults for zero values
	if config.Port == 0 {
		config.Port = 22
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.TermType == "" {
		config.TermType = "xterm-256color"
	}
	if config.Cols == 0 {
		config.Cols = 80
	}
	if config.Rows == 0 {
		config.Rows = 24
	}

	return &SSHBackend{
		config: config,
		state:  StateDisconnected,
		ctx:    ctx,
		cancel: cancel,
	}
}

// SetAuthPromptHandler sets the callback for keyboard-interactive prompts
func (s *SSHBackend) SetAuthPromptHandler(handler AuthPromptCallback) {
	s.authPromptHandler = handler
}

// SetStateChangeHandler sets the callback for state changes
func (s *SSHBackend) SetStateChangeHandler(handler StateChangeCallback) {
	s.stateChangeHandler = handler
}

// setState updates the connection state and notifies listeners
func (s *SSHBackend) setState(newState ConnectionState) {
	s.stateMutex.Lock()
	oldState := s.state
	s.state = newState
	s.stateMutex.Unlock()

	if oldState != newState {
		log.Printf("SSH state change: %s -> %s", oldState, newState)
		if s.stateChangeHandler != nil {
			s.stateChangeHandler(oldState, newState)
		}
	}
}

// GetState returns the current connection state
func (s *SSHBackend) GetState() ConnectionState {
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()
	return s.state
}

// GetLastError returns the last error that occurred
func (s *SSHBackend) GetLastError() error {
	return s.lastError
}

// GetAuthMethod returns the authentication method that succeeded
func (s *SSHBackend) GetAuthMethod() AuthMethod {
	return s.authMethod
}

// Connect establishes the SSH connection
func (s *SSHBackend) Connect() error {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()

	if s.state == StateConnected {
		return nil // Already connected
	}

	s.setState(StateConnecting)

	// Build SSH client config
	clientConfig, err := s.buildClientConfig()
	if err != nil {
		s.lastError = fmt.Errorf("failed to build SSH config: %w", err)
		s.setState(StateError)
		return s.lastError
	}

	// Connect to server
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	log.Printf("SSH: Connecting to %s as %s", addr, s.config.Username)

	conn, err := net.DialTimeout("tcp", addr, s.config.Timeout)
	if err != nil {
		s.lastError = fmt.Errorf("failed to connect to %s: %w", addr, err)
		s.setState(StateError)
		return s.lastError
	}

	// Set connection deadline for SSH handshake
	conn.SetDeadline(time.Now().Add(s.config.Timeout))

	s.setState(StateAuthenticating)

	// Perform SSH handshake
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, clientConfig)
	if err != nil {
		conn.Close()
		s.lastError = fmt.Errorf("SSH handshake failed: %w", err)
		s.setState(StateError)
		return s.lastError
	}

	// Clear deadline after successful handshake
	conn.SetDeadline(time.Time{})

	// Create SSH client
	s.client = ssh.NewClient(sshConn, chans, reqs)

	// Create session
	if err := s.createSession(); err != nil {
		s.client.Close()
		s.client = nil
		s.lastError = err
		s.setState(StateError)
		return err
	}

	s.setState(StateConnected)

	// Start keepalive if configured
	if s.config.KeepAliveInterval > 0 {
		s.startKeepAlive()
	}

	log.Printf("SSH: Connected successfully via %s", s.authMethod)
	return nil
}

// buildClientConfig creates the SSH client configuration with auth methods
func (s *SSHBackend) buildClientConfig() (*ssh.ClientConfig, error) {
	// Build authentication methods in priority order
	authMethods, err := s.buildAuthMethods()
	if err != nil {
		return nil, err
	}

	if len(authMethods) == 0 {
		return nil, errors.New("no authentication methods available")
	}

	// Build host key callback
	hostKeyCallback, err := s.buildHostKeyCallback()
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User:            s.config.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         s.config.Timeout,
		// Support modern key exchange algorithms
		Config: ssh.Config{
			KeyExchanges: []string{
				"curve25519-sha256",
				"curve25519-sha256@libssh.org",
				"ecdh-sha2-nistp256",
				"ecdh-sha2-nistp384",
				"ecdh-sha2-nistp521",
				"diffie-hellman-group14-sha256",
				"diffie-hellman-group14-sha1",
			},
		},
	}

	return config, nil
}

// buildAuthMethods creates the list of authentication methods to try
func (s *SSHBackend) buildAuthMethods() ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	// 1. SSH Agent (if enabled and available)
	if s.config.UseAgent {
		if agentAuth := s.getAgentAuth(); agentAuth != nil {
			methods = append(methods, agentAuth)
			log.Printf("SSH: Added SSH agent authentication")
		}
	}

	// 2. Public key authentication
	if len(s.config.PrivateKey) > 0 || s.config.PrivateKeyPath != "" {
		if keyAuth, err := s.getPublicKeyAuth(); err == nil && keyAuth != nil {
			methods = append(methods, keyAuth)
			log.Printf("SSH: Added public key authentication")
		} else if err != nil {
			log.Printf("SSH: Public key auth setup failed: %v", err)
		}
	}

	// 3. Password authentication
	if s.config.Password != "" {
		methods = append(methods, ssh.Password(s.config.Password))
		log.Printf("SSH: Added password authentication")
	}

	// 4. Keyboard-interactive (handles MFA, RADIUS, etc.)
	// Always add this as it can handle password prompts too
	methods = append(methods, ssh.KeyboardInteractive(s.keyboardInteractiveCallback))
	log.Printf("SSH: Added keyboard-interactive authentication")

	return methods, nil
}

// getAgentAuth returns SSH agent authentication if available
func (s *SSHBackend) getAgentAuth() ssh.AuthMethod {
	// Get SSH_AUTH_SOCK from environment
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil
	}

	conn, err := net.Dial("unix", socket)
	if err != nil {
		log.Printf("SSH: Could not connect to SSH agent: %v", err)
		return nil
	}

	// Note: We're not closing conn here - the agent connection
	// needs to stay open for the auth to work. It will be cleaned
	// up when the process exits.
	agentClient := ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
		// This is a simplified version - for production you'd use
		// golang.org/x/crypto/ssh/agent package
		return nil, errors.New("agent not fully implemented")
	})

	// For now, return nil - full agent support requires the agent package
	_ = conn
	_ = agentClient
	return nil
}

// getPublicKeyAuth returns public key authentication
func (s *SSHBackend) getPublicKeyAuth() (ssh.AuthMethod, error) {
	var keyData []byte
	var err error

	// Prefer in-memory key over file path
	if len(s.config.PrivateKey) > 0 {
		keyData = s.config.PrivateKey
	} else if s.config.PrivateKeyPath != "" {
		// Expand ~ in path
		keyPath := s.config.PrivateKeyPath
		if strings.HasPrefix(keyPath, "~/") {
			homeDir, _ := os.UserHomeDir()
			keyPath = filepath.Join(homeDir, keyPath[2:])
		}

		keyData, err = os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key %s: %w", keyPath, err)
		}
	} else {
		return nil, nil
	}

	// Parse the private key
	var signer ssh.Signer

	if s.config.KeyPassphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(s.config.KeyPassphrase))
	} else {
		signer, err = ssh.ParsePrivateKey(keyData)
	}

	if err != nil {
		// Check if key is encrypted but no passphrase provided
		if strings.Contains(err.Error(), "encrypted") {
			// Try to get passphrase via prompt
			if s.authPromptHandler != nil {
				passphrase, promptErr := s.authPromptHandler("Enter passphrase for private key:", false)
				if promptErr != nil {
					return nil, fmt.Errorf("failed to get key passphrase: %w", promptErr)
				}
				signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(passphrase))
				if err != nil {
					return nil, fmt.Errorf("failed to parse private key with passphrase: %w", err)
				}
			} else {
				return nil, fmt.Errorf("private key is encrypted but no passphrase provided")
			}
		} else {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
	}

	return ssh.PublicKeys(signer), nil
}

// keyboardInteractiveCallback handles keyboard-interactive authentication
func (s *SSHBackend) keyboardInteractiveCallback(user, instruction string, questions []string, echos []bool) ([]string, error) {
	log.Printf("SSH: Keyboard-interactive auth: user=%s, instruction=%q, questions=%d",
		user, instruction, len(questions))

	answers := make([]string, len(questions))

	for i, question := range questions {
		questionLower := strings.ToLower(question)
		log.Printf("SSH: Question %d: %q (echo=%v)", i, question, echos[i])

		// Try to answer automatically if we have credentials
		if strings.Contains(questionLower, "password") && s.config.Password != "" {
			answers[i] = s.config.Password
			log.Printf("SSH: Auto-answered password question")
			continue
		}

		// For other questions, use the prompt handler
		if s.authPromptHandler != nil {
			answer, err := s.authPromptHandler(question, echos[i])
			if err != nil {
				return nil, fmt.Errorf("failed to get answer for %q: %w", question, err)
			}
			answers[i] = answer
		} else {
			// No handler and can't auto-answer
			return nil, fmt.Errorf("no handler for keyboard-interactive question: %q", question)
		}
	}

	return answers, nil
}

// buildHostKeyCallback creates the host key verification callback
func (s *SSHBackend) buildHostKeyCallback() (ssh.HostKeyCallback, error) {
	// Use custom callback if provided
	if s.config.HostKeyCallback != nil {
		return s.config.HostKeyCallback, nil
	}

	// DANGER: Skip verification if explicitly requested
	if s.config.InsecureIgnoreKey {
		log.Printf("SSH: WARNING - Host key verification disabled!")
		return ssh.InsecureIgnoreHostKey(), nil
	}

	// Use known_hosts file
	if s.config.KnownHostsPath != "" {
		// Check if file exists
		if _, err := os.Stat(s.config.KnownHostsPath); os.IsNotExist(err) {
			// Create empty known_hosts file
			dir := filepath.Dir(s.config.KnownHostsPath)
			if err := os.MkdirAll(dir, 0700); err != nil {
				return nil, fmt.Errorf("failed to create .ssh directory: %w", err)
			}
			if err := os.WriteFile(s.config.KnownHostsPath, []byte{}, 0600); err != nil {
				return nil, fmt.Errorf("failed to create known_hosts file: %w", err)
			}
		}

		callback, err := knownhosts.New(s.config.KnownHostsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load known_hosts: %w", err)
		}

		// Wrap to handle unknown hosts
		return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			err := callback(hostname, remote, key)
			if err != nil {
				// Check if it's a key not found error
				var keyErr *knownhosts.KeyError
				if errors.As(err, &keyErr) && len(keyErr.Want) == 0 {
					// Unknown host - prompt user or auto-accept based on config
					log.Printf("SSH: Unknown host key for %s", hostname)
					// For now, log and accept - in production you'd prompt
					// TODO: Add host key acceptance callback
					return nil
				}
			}
			return err
		}, nil
	}

	// Fallback to insecure if no other option
	log.Printf("SSH: WARNING - No known_hosts file, using insecure host key verification")
	return ssh.InsecureIgnoreHostKey(), nil
}

// createSession creates an SSH session with PTY
func (s *SSHBackend) createSession() error {
	session, err := s.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}

	// Request PTY
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     // Enable echo
		ssh.TTY_OP_ISPEED: 14400, // Input speed
		ssh.TTY_OP_OSPEED: 14400, // Output speed
		ssh.VINTR:         3,     // Ctrl+C
		ssh.VQUIT:         28,    // Ctrl+\
		ssh.VERASE:        127,   // Backspace
		ssh.VKILL:         21,    // Ctrl+U
		ssh.VEOF:          4,     // Ctrl+D
		ssh.VSUSP:         26,    // Ctrl+Z
	}

	if err := session.RequestPty(s.config.TermType, s.config.Rows, s.config.Cols, modes); err != nil {
		session.Close()
		return fmt.Errorf("failed to request PTY: %w", err)
	}

	// Get stdin pipe
	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	// Get stdout and stderr
	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		session.Close()
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Create combined output pipe
	s.outputReader, s.outputWriter = io.Pipe()

	// Merge stdout and stderr into single output
	go func() {
		io.Copy(s.outputWriter, stdout)
	}()
	go func() {
		io.Copy(s.outputWriter, stderr)
	}()

	// Start shell
	if err := session.Shell(); err != nil {
		session.Close()
		return fmt.Errorf("failed to start shell: %w", err)
	}

	s.session = session
	s.stdin = stdin
	s.stdout = stdout
	s.stderr = stderr

	// Monitor session for unexpected close
	go s.monitorSession()

	return nil
}

// monitorSession watches for session termination
func (s *SSHBackend) monitorSession() {
	if s.session == nil {
		return
	}

	err := s.session.Wait()
	log.Printf("SSH: Session ended: %v", err)

	// Only update state if we haven't already disconnected
	if s.GetState() == StateConnected {
		s.lastError = err
		s.setState(StateDisconnected)
	}

	// Close the output writer to signal EOF to readers
	if s.outputWriter != nil {
		s.outputWriter.Close()
	}
}

// startKeepAlive starts the keepalive goroutine
func (s *SSHBackend) startKeepAlive() {
	s.keepAliveDone = make(chan struct{})

	go func() {
		ticker := time.NewTicker(s.config.KeepAliveInterval)
		defer ticker.Stop()

		missedCount := 0

		for {
			select {
			case <-ticker.C:
				if s.client == nil {
					return
				}

				// Send keepalive request
				_, _, err := s.client.SendRequest("keepalive@openssh.com", true, nil)
				if err != nil {
					missedCount++
					log.Printf("SSH: Keepalive failed (%d/%d): %v",
						missedCount, s.config.KeepAliveMaxCount, err)

					if missedCount >= s.config.KeepAliveMaxCount {
						log.Printf("SSH: Too many missed keepalives, disconnecting")
						s.Close()
						return
					}
				} else {
					missedCount = 0
				}

			case <-s.keepAliveDone:
				return
			case <-s.ctx.Done():
				return
			}
		}
	}()
}

// Read implements TerminalBackend.Read
func (s *SSHBackend) Read(p []byte) (n int, err error) {
	if s.outputReader == nil {
		return 0, io.EOF
	}
	return s.outputReader.Read(p)
}

// Write implements TerminalBackend.Write
func (s *SSHBackend) Write(p []byte) (n int, err error) {
	if s.stdin == nil {
		return 0, errors.New("not connected")
	}
	return s.stdin.Write(p)
}

// Resize implements TerminalBackend.Resize
func (s *SSHBackend) Resize(cols, rows int) error {
	if s.session == nil {
		return errors.New("not connected")
	}

	// Update config
	s.config.Cols = cols
	s.config.Rows = rows

	log.Printf("SSHBackend.Resize: sending WindowChange(%d rows, %d cols)", rows, cols)

	// Send window change request
	return s.session.WindowChange(rows, cols)
}

// Close implements TerminalBackend.Close
func (s *SSHBackend) Close() error {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()

	// Signal keepalive to stop
	if s.keepAliveDone != nil {
		close(s.keepAliveDone)
		s.keepAliveDone = nil
	}

	// Cancel context
	s.cancel()

	// Close session
	if s.session != nil {
		s.session.Close()
		s.session = nil
	}

	// Close client
	if s.client != nil {
		s.client.Close()
		s.client = nil
	}

	// Close pipes
	if s.outputWriter != nil {
		s.outputWriter.Close()
		s.outputWriter = nil
	}
	if s.outputReader != nil {
		s.outputReader.Close()
		s.outputReader = nil
	}

	s.stdin = nil
	s.stdout = nil
	s.stderr = nil

	s.setState(StateDisconnected)
	return nil
}

// IsConnected implements TerminalBackend.IsConnected
func (s *SSHBackend) IsConnected() bool {
	return s.GetState() == StateConnected
}

// GetHostKey returns the host key of the connected server
func (s *SSHBackend) GetHostKey() ssh.PublicKey {
	if s.client == nil {
		return nil
	}
	return s.client.Conn.RemoteAddr().(interface{ PublicKey() ssh.PublicKey }).PublicKey()
}

// SendSignal sends a signal to the remote process
func (s *SSHBackend) SendSignal(sig ssh.Signal) error {
	if s.session == nil {
		return errors.New("not connected")
	}
	return s.session.Signal(sig)
}

// ============================================================================
// SSHTerminalWidget - Combines SSHBackend with NativeTerminalWidget
// ============================================================================

// SSHTerminalWidget wraps NativeTerminalWidget with SSH connectivity
type SSHTerminalWidget struct {
	*NativeTerminalWidget

	// SSH backend
	sshBackend *SSHBackend
	sshConfig  SSHConfig

	// State callbacks
	onStateChange func(ConnectionState)
	onError       func(error)
	stream        *gopyte.Stream
	cancelRead    context.CancelFunc

	// Auth UI callback - implement this in your session manager
	// to show password dialogs, MFA prompts, etc.
	authUIHandler AuthPromptCallback
}

// NewSSHTerminalWidget creates a new SSH-enabled terminal widget
// *** UPDATED: Now sets up resize callback for proper SSH resize propagation ***
func NewSSHTerminalWidget(darkMode bool) *SSHTerminalWidget {
	w := &SSHTerminalWidget{
		NativeTerminalWidget: NewNativeTerminalWidget(darkMode),
	}

	// Override the write function to route through SSH when connected
	w.NativeTerminalWidget.writeOverride = func(data []byte) {
		log.Printf("writeOverride CALLED with %d bytes", len(data))
		if w.sshBackend != nil && w.sshBackend.IsConnected() {
			_, err := w.sshBackend.Write(data)
			if err != nil {
				log.Printf("SSH write error: %v", err)
			}
		} else {
			log.Printf("writeOverride: sshBackend nil or not connected")
		}
	}

	// *** FIX: Set up resize callback to propagate resize events to SSH session ***
	w.NativeTerminalWidget.SetResizeCallback(func(cols, rows int) {
		log.Printf("SSH resize callback triggered: %dx%d", cols, rows)
		w.ResizeTerminal(cols, rows)
	})

	log.Printf("NewSSHTerminalWidget: writeOverride and resizeCallback configured")

	return w
}

// SetSSHConfig sets the SSH connection configuration
func (w *SSHTerminalWidget) SetSSHConfig(config SSHConfig) {
	w.sshConfig = config
}

// SetAuthUIHandler sets the callback for authentication prompts
// This should be implemented by your session manager to show dialogs
func (w *SSHTerminalWidget) SetAuthUIHandler(handler AuthPromptCallback) {
	w.authUIHandler = handler
}

// SetStateChangeHandler sets the callback for connection state changes
func (w *SSHTerminalWidget) SetStateChangeHandler(handler func(ConnectionState)) {
	w.onStateChange = handler
}

// SetErrorHandler sets the callback for connection errors
func (w *SSHTerminalWidget) SetErrorHandler(handler func(error)) {
	w.onError = handler
}

// ConnectSSH establishes the SSH connection
func (w *SSHTerminalWidget) ConnectSSH() error {
	// Create SSH backend
	w.sshBackend = NewSSHBackend(w.sshConfig)

	// Set up auth prompt handler
	w.sshBackend.SetAuthPromptHandler(w.authUIHandler)

	// Set up state change handler
	w.sshBackend.SetStateChangeHandler(func(oldState, newState ConnectionState) {
		if w.onStateChange != nil {
			w.onStateChange(newState)
		}
	})

	// Connect
	if err := w.sshBackend.Connect(); err != nil {
		if w.onError != nil {
			w.onError(err)
		}
		return err
	}

	// Create gopyte stream for feeding SSH output
	if w.screen == nil {
		return fmt.Errorf("screen not initialized")
	}
	w.stream = gopyte.NewStream(w.screen, false) // false = parse ANSI

	// Start reading from SSH and feeding to gopyte
	go w.sshReadLoop()

	// CRITICAL FIX: Trigger resize after connection to sync terminal size
	// The widget may already be sized, but SSH was started with default 80x24
	go w.triggerPostConnectResize()

	return nil
}

// triggerPostConnectResize sends the actual terminal size to the SSH session
// after connection is established. This fixes the initial size mismatch.
func (w *SSHTerminalWidget) triggerPostConnectResize() {
	// Small delay to let the connection fully establish and widget render
	time.Sleep(100 * time.Millisecond)

	fyne.Do(func() {
		// Get the widget's actual size
		size := w.Size()
		if size.Width > 0 && size.Height > 0 {
			// Calculate terminal dimensions from widget size
			cols, rows := w.CalculateTerminalSize(size.Width, size.Height)

			log.Printf("Post-connect resize: widget=%.0fx%.0f -> terminal=%dx%d",
				size.Width, size.Height, cols, rows)

			// Resize everything: local screen, gopyte, and SSH session
			w.ResizeTerminal(cols, rows)
		} else {
			log.Printf("Post-connect resize: widget size not available yet, using defaults")
		}
	})
}

// sshReadLoop - Reads from SSH and feeds to gopyte for terminal emulation
func (w *SSHTerminalWidget) sshReadLoop() {
	ctx, cancel := context.WithCancel(context.Background())
	w.cancelRead = cancel
	defer cancel()

	buf := make([]byte, 64*1024)

	for {
		select {
		case <-ctx.Done():
			log.Printf("sshReadLoop: cancelled via cancelRead")
			return
		case <-w.sshBackend.ctx.Done():
			log.Printf("sshReadLoop: backend context done")
			return
		default:
			n, err := w.sshBackend.Read(buf)
			if err != nil {
				if err == io.EOF {
					log.Printf("sshReadLoop: EOF from server")
				} else if !strings.Contains(err.Error(), "closed") && !errors.Is(err, context.Canceled) {
					log.Printf("SSH read error: %v", err)
					if w.onError != nil {
						fyne.Do(func() { w.onError(err) })
					}
				}
				return
			}

			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])

				// Feed to gopyte for terminal emulation
				if w.stream != nil {
					w.stream.Feed(string(data))
				}

				// Trigger redraw + auto-scroll
				w.updatePending = true
				fyne.Do(func() {
					if w.textGrid != nil {
						w.performRedrawDirect()
					}
					if w.screen != nil && !w.screen.IsUsingAlternate() && !w.screen.IsViewingHistory() {
						w.screen.ScrollToBottom()
					}
				})
			}
		}
	}
}

// DisconnectWithContext - for graceful app shutdown (used by SessionManager.DisconnectAll)
func (w *SSHTerminalWidget) DisconnectWithContext(ctx context.Context) {
	// Capture backend FIRST, before anything can nil it
	backend := w.sshBackend

	// Cancel read loop immediately
	if w.cancelRead != nil {
		w.cancelRead()
		w.cancelRead = nil
	}

	// If no backend, we're already done
	if backend == nil {
		log.Printf("DisconnectWithContext: backend already nil, skipping")
		return
	}

	// Close backend in background with timeout
	done := make(chan error, 1)
	go func(b *SSHBackend) {
		log.Printf("Starting backend.Close() for %s", w.sshConfig.Host)
		done <- b.Close()
	}(backend) // Pass by value - safe from nil

	select {
	case err := <-done:
		if err != nil && !strings.Contains(err.Error(), "session closed") {
			log.Printf("SSH close error: %v", err)
		} else {
			log.Printf("SSH session closed cleanly")
		}
	case <-ctx.Done():
		log.Printf("SSH close timed out (forced) for %s", w.sshConfig.Host)
	}

	// Only NOW do we nil things out
	w.sshBackend = nil
	w.stream = nil
}

func (w *SSHTerminalWidget) Disconnect() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	w.DisconnectWithContext(ctx)
}

func (w *SSHTerminalWidget) Close() {
	w.Disconnect()
	if w.NativeTerminalWidget != nil {
		w.NativeTerminalWidget.Close()
	}
}

// WriteToPTY overrides the parent method to write to SSH instead
func (w *SSHTerminalWidget) WriteToPTY(data []byte) error {
	if w.sshBackend != nil && w.sshBackend.IsConnected() {
		_, err := w.sshBackend.Write(data)
		if err != nil {
			log.Printf("SSH write error: %v", err)
			return err
		}
		return nil
	}
	// Fall back to local PTY if no SSH connection
	return w.NativeTerminalWidget.WriteToPTY(data)
}

// ResizeTerminal resizes both the local screen and SSH session
// *** UPDATED: Now also resizes gopyte screen ***
func (w *SSHTerminalWidget) ResizeTerminal(cols, rows int) {
	log.Printf("SSHTerminalWidget.ResizeTerminal: %dx%d", cols, rows)

	// Update local dimensions
	w.NativeTerminalWidget.cols = cols
	w.NativeTerminalWidget.rows = rows

	// Resize gopyte screen if it exists
	if w.NativeTerminalWidget.screen != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Error resizing gopyte screen: %v", r)
				}
			}()
			w.NativeTerminalWidget.screen.Resize(cols, rows)
			log.Printf("Gopyte screen resized to %dx%d", cols, rows)
		}()
	}

	// Resize SSH session (sends WindowChange to remote)
	if w.sshBackend != nil && w.sshBackend.IsConnected() {
		if err := w.sshBackend.Resize(cols, rows); err != nil {
			log.Printf("SSH resize error: %v", err)
		} else {
			log.Printf("SSH session resized to %dx%d", cols, rows)
		}
	}

	// Force redraw
	w.updatePending = true
}

// GetSSHState returns the current SSH connection state
func (w *SSHTerminalWidget) GetSSHState() ConnectionState {
	if w.sshBackend == nil {
		return StateDisconnected
	}
	return w.sshBackend.GetState()
}

// IsSSHConnected returns true if SSH is connected
func (w *SSHTerminalWidget) IsSSHConnected() bool {
	return w.sshBackend != nil && w.sshBackend.IsConnected()
}
