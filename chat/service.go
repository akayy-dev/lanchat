package chat

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"
)

// connWrapper adds per-connection write mutex to prevent message frame corruption.
// When multiple goroutines call Broadcast() concurrently, their writes to the same
// connection can interleave, corrupting the 4-byte length header + payload framing.
// This wrapper ensures only one write operation happens at a time per connection.
type connWrapper struct {
	conn    net.Conn
	writeMu sync.Mutex // Protects conn.Write operations
}

// used to differentiate between regular messages and other types (like exchange)
type TCPMessageType string

const (
	// Used to announce presence to listeners, and to exchange public keys for encryption.
	PUBKEY_HANDSHAKE TCPMessageType = "pubkey_handshake"
	// Used to send the sender key to a peer after the public key exchange.
	SENDER_KEY TCPMessageType = "sender_key"
	// Regular chat message
	CHAT_MESSAGE   TCPMessageType = "chat"
	maxMessageSize                = 1 << 20 // 1 MiB safety cap for framed payloads.
)

type TCPMessage struct {
	Type      TCPMessageType `json:"type"`
	From      string         `json:"from"`
	Content   []byte         `json:"content"`
	Timestamp time.Time      `json:"timestamp"`
}

func NewChatService(localPeerID string) *ChatService {
	return &ChatService{
		LocalPeerID:         localPeerID,
		connections:         make(map[string]*connWrapper),
		peers:               make(map[string]Peer),
		SendMessageChan:     make(chan string, 20),     // Buffered channel for sending messages
		ReceivedMessageChan: make(chan TCPMessage, 20), // Buffered channel for receiving messages
		ErrorChan:           make(chan error, 20),      // Buffered channel for errors
		EncryptionService:   *NewSenderKeyEncryptor(),
	}
}

type ChatService struct {
	LocalPeerID string
	// Each connWrapper contains a mutex to serialize writes to that specific connection,
	// preventing message frame corruption from concurrent Broadcast() calls.
	connections       map[string]*connWrapper
	listener          net.Listener
	peers             map[string]Peer
	mu                sync.RWMutex
	EncryptionService SenderKeyEncryptor

	// Channels for sending and receiving updates
	SendMessageChan     chan string     // UI will send messages to this channel to broadcast to all peers
	ReceivedMessageChan chan TCPMessage // Received messages from peers will be sent to this channel for the UI to display
	ErrorChan           chan error      // Errors encountered during networking operations will be sent to this channel for logging or UI display
}

// dialPeer attempts to establish a TCP connection to the given peer.
func (c *ChatService) dialPeer(peer Peer) (net.Conn, error) {
	return net.DialTimeout("tcp", peer.Addr, 5*time.Second)
}

func (c *ChatService) Start(listener net.Listener) {
	c.mu.Lock()
	c.listener = listener
	c.mu.Unlock()
	go c.acceptLoop()
	go c.sendMessageLoop()
}

// Goroutine to continuously read messages from the SendMessageChan and broadcast them to all connected peers.
func (c *ChatService) sendMessageLoop() {
	for content := range c.SendMessageChan {
		err := c.Broadcast(content)
		if err != nil {
			slog.Error("Failed to broadcast message", slog.Any("err", err))
			// FIX: Use non-blocking send to ErrorChan to prevent deadlock.
			// If the error channel is full, we log and continue rather than blocking forever.
			select {
			case c.ErrorChan <- err:
				// Error sent to channel
			default:
				slog.Warn("ErrorChan full, dropping error", "err", err)
			}
		}
	}
}

// Close gracefully shuts down the ChatService, closing all active connections and the listener.
func (c *ChatService) Close() {
	c.mu.Lock()
	// Snapshot of active connections (using connWrapper now)
	conns := make([]*connWrapper, 0, len(c.connections))
	for peerID, wrapper := range c.connections {
		conns = append(conns, wrapper)
		delete(c.connections, peerID)
	}
	listener := c.listener
	c.listener = nil
	c.mu.Unlock()

	// Close channels to signal other goroutines to stop.
	close(c.SendMessageChan)
	close(c.ReceivedMessageChan)
	close(c.ErrorChan)

	// Close all active connections
	for _, wrapper := range conns {
		wrapper.conn.Close()
	}

	// Close the listener if it's still open
	if listener != nil {
		listener.Close()
	}
}

// acceptLoop continuously accepts incoming TCP connections and handles them.
func (c *ChatService) acceptLoop() {
	for {
		c.mu.RLock()
		listener := c.listener
		c.mu.RUnlock()

		if listener == nil {
			return
		}

		conn, err := listener.Accept()
		if err != nil {
			// If listener was closed during shutdown, exit gracefully.
			c.mu.RLock()
			isShuttingDown := c.listener == nil
			c.mu.RUnlock()
			if isShuttingDown {
				return
			}

			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Temporary() {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			slog.Warn("Accept failed", "err", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		go c.handleIncomingConnection(conn)
	}
}

// handlePublicKeyExchange handles the public key exchange handshake with a peer.
// It registers the peer's public key, computes the shared secret, derives the sender key,
// and sends both our public key and sender key back to the peer.
// isResponder parameter indicates if we are responding to an initial handshake (true)
// or if we initiated the connection and are receiving a response (false).
func (c *ChatService) handlePublicKeyExchange(peerID string, conn net.Conn, remotePublicKey []byte, isResponder bool) error {
	// Register the peer and compute shared secret
	sharedSecret, err := c.EncryptionService.RegisterPeer(peerID, remotePublicKey)
	if err != nil {
		return err
	}

	// Derive and store our sender key for this peer
	senderKey := c.EncryptionService.DeriveSenderKey(sharedSecret)
	c.EncryptionService.SetSenderKey(peerID, senderKey)

	// If we're the responder (acceptor), we need to send our public key back
	if isResponder {
		if err := c.SendMessage(conn, TCPMessage{
			Type:      PUBKEY_HANDSHAKE,
			From:      c.LocalPeerID,
			Content:   c.EncryptionService.PublicKey.Bytes(),
			Timestamp: time.Now(),
		}); err != nil {
			return err
		}
	}

	// Encrypt the sender key with the shared secret before sending
	// This ensures the sender key is never transmitted in plaintext
	encryptedSenderKey, err := c.EncryptionService.EncryptMessageWithSharedSecret(peerID, senderKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt sender key: %w", err)
	}

	// Send our encrypted sender key so the peer can decrypt messages from us
	if err := c.SendMessage(conn, TCPMessage{
		Type:      SENDER_KEY,
		From:      c.LocalPeerID,
		Content:   encryptedSenderKey,
		Timestamp: time.Now(),
	}); err != nil {
		return err
	}

	return nil
}

func (c *ChatService) handleIncomingConnection(conn net.Conn) {
	message, err := c.readMessage(conn)
	if err != nil {
		slog.Error("Failed to read message", slog.Any("err", err))
		conn.Close()
		return
	}

	// protect against malformed requests and messages sent from self
	peerID := message.From
	if peerID == "" || peerID == c.LocalPeerID {
		conn.Close()
		return
	}

	// FIX: Wrap connection in connWrapper for per-connection write mutex
	wrapper := &connWrapper{conn: conn}

	c.mu.Lock()
	if oldWrapper, exists := c.connections[peerID]; exists && oldWrapper.conn != conn {
		oldWrapper.conn.Close()
	}
	c.connections[peerID] = wrapper // Store the wrapped connection for future communication
	c.mu.Unlock()

	// Process the initial message (should be PUBKEY_HANDSHAKE from the dialer)
	if message.Type == PUBKEY_HANDSHAKE {
		slog.Info("Got public key (initial)")
		// We are the responder (acceptor) - send our pubkey back
		if err := c.handlePublicKeyExchange(peerID, conn, message.Content, true); err != nil {
			slog.Error("Failed to exchange public keys", slog.Any("err", err))
			conn.Close()
			return
		}
	}

	c.readLoop(peerID, wrapper)
}

// readLoop now accepts connWrapper instead of raw net.Conn
func (c *ChatService) readLoop(peerID string, wrapper *connWrapper) {
	// when the connection is closed, remove it from the service
	defer wrapper.conn.Close()
	defer func() {
		c.mu.Lock()
		if existingWrapper, ok := c.connections[peerID]; ok && existingWrapper == wrapper {
			delete(c.connections, peerID)
		}
		c.mu.Unlock()
	}()

	for {
		message, err := c.readMessage(wrapper.conn)
		if err != nil {
			return
		}

		switch message.Type {
		case PUBKEY_HANDSHAKE:
			slog.Debug("Got public key")
			// We are the initiator (dialer) receiving a response - don't send pubkey again
			if err := c.handlePublicKeyExchange(peerID, wrapper.conn, message.Content, false); err != nil {
				slog.Error("Failed to exchange public keys", slog.Any("err", err))
				return
			}
		case SENDER_KEY:
			slog.Debug("Got sender key (encrypted)")
			// Decrypt the sender key using the shared secret
			decryptedSenderKey, err := c.EncryptionService.DecryptMessageWithSharedSecret(peerID, message.Content)
			if err != nil {
				slog.Error("Failed to decrypt sender key", slog.Any("err", err))
				return
			}
			c.EncryptionService.SetSenderKey(peerID, decryptedSenderKey)
		case CHAT_MESSAGE:
			// decrypt message with sender key before forwarding to UI
			decryptedContent, err := c.EncryptionService.DecryptMessageWithSenderKey(peerID, message.Content)
			decryptedContent = message.Content
			if err != nil {
				slog.Error("Failed to decrypt message", slog.Any("err", err))
				continue
			}
			message.Content = decryptedContent
			select {
			case c.ReceivedMessageChan <- message:
				// Message forwarded to UI
			default:
				slog.Warn("ReceivedMessageChan full, dropping message", "from", message.From)
			}
		}
	}
}

// readMessage reads a TCPMessage from the given connection.
func (c *ChatService) readMessage(conn net.Conn) (TCPMessage, error) {
	var header [4]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return TCPMessage{}, err
	}
	payloadLength := binary.BigEndian.Uint32(header[:])
	if payloadLength > maxMessageSize {
		return TCPMessage{}, fmt.Errorf("payload too large: %d bytes", payloadLength)
	}

	payload := make([]byte, payloadLength)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return TCPMessage{}, err
	}

	var message TCPMessage
	if err := json.Unmarshal(payload, &message); err != nil {
		return TCPMessage{}, err
	}
	return message, nil
}

// SendMessage sends a TCPMessage to a raw connection (used during initial handshake).
// For regular messaging, use SendMessageToWrapper which provides write mutex protection.
func (c *ChatService) SendMessage(conn net.Conn, message TCPMessage) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}

	// CREATE FRAME: HEADER + PAYLOAD
	frame := make([]byte, 4+len(payload))
	// the header contains the length of the payload, so the receiver knows how many bytes to expect.
	binary.BigEndian.PutUint32(frame[:4], uint32(len(payload)))
	copy(frame[4:], payload) // put the payload after the header

	for len(frame) > 0 {
		// Write payload
		n, err := conn.Write(frame)
		if err != nil {
			return err
		}

		if n == 0 {
			return io.ErrShortWrite
		}

		// shift frame to next element in the array
		frame = frame[n:]
	}
	slog.Info("Sent message", "type", message.Type, "to", conn.RemoteAddr().String())
	return nil
}

// FIX: SendMessageToWrapper sends a message using the connWrapper's write mutex.
// This prevents message frame corruption when multiple goroutines call Broadcast() concurrently.
// The mutex ensures the entire frame (4-byte header + payload) is written atomically.
func (c *ChatService) SendMessageToWrapper(wrapper *connWrapper, message TCPMessage) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}

	// CREATE FRAME: HEADER + PAYLOAD
	frame := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(frame[:4], uint32(len(payload)))
	copy(frame[4:], payload)

	// Lock the mutex so connection only writes one message at a time.
	wrapper.writeMu.Lock()
	defer wrapper.writeMu.Unlock()

	for len(frame) > 0 {
		n, err := wrapper.conn.Write(frame)
		if err != nil {
			return err
		}

		if n == 0 {
			return io.ErrShortWrite
		}

		frame = frame[n:]
	}
	slog.Info("Sent message", "type", message.Type, "to", wrapper.conn.RemoteAddr().String())
	return nil
}

// Broadcast sends a chat message to all connected peers.
func (c *ChatService) Broadcast(content string) error {
	c.mu.RLock()
	// Build snapshot of connections
	snapshot := make(map[string]*connWrapper, len(c.connections))
	for peerID, wrapper := range c.connections {
		snapshot[peerID] = wrapper
	}
	c.mu.RUnlock()

	var errs []error
	var failedPeers []string

	for peerID, wrapper := range snapshot {
		if peerID == c.LocalPeerID {
			continue // Skip sending to self
		}

		// encrypt the message with the sender key for this peer
		encryptedContent, err := c.EncryptionService.EncryptMessageWithSenderKey(peerID, []byte(content))
		if err != nil {
			slog.Error("Failed to encrypt message for peer", "peer", peerID, "err", err)
			errs = append(errs, fmt.Errorf("peer %s: %w", peerID, err))
			failedPeers = append(failedPeers, peerID)
			continue
		}

		err = c.SendMessageToWrapper(wrapper, TCPMessage{
			Type:      CHAT_MESSAGE,
			From:      c.LocalPeerID,
			Content:   encryptedContent,
			Timestamp: time.Now(),
		})
		if err != nil {
			slog.Error("Failed to send message to peer", "peer", peerID, "err", err)
			errs = append(errs, fmt.Errorf("peer %s: %w", peerID, err))
			failedPeers = append(failedPeers, peerID)
		}
	}

	// FIX: Clean up dead connections to prevent repeated failures.
	// We do this after the loop to avoid modifying the map while iterating over snapshot.
	if len(failedPeers) > 0 {
		c.mu.Lock()
		for _, peerID := range failedPeers {
			if wrapper, ok := c.connections[peerID]; ok {
				// Only remove if it's the same connection (could have been replaced)
				if snapshot[peerID] == wrapper {
					slog.Info("Removing dead connection", "peer", peerID)
					delete(c.connections, peerID)
					wrapper.conn.Close()
				}
			}
		}
		c.mu.Unlock()
	}

	// FIX: Return aggregated errors so callers know about failures
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// RegisterPeer registers a new peer and establishes a connection if the local peer ID is lower than the remote peer ID.
func (c *ChatService) RegisterPeer(peer Peer) error {
	if c.LocalPeerID == peer.PeerID {
		// Ignore self
		return nil
	}

	// The peer with the lower ID will Dial the other,
	// to prevent both peers from trying to Dial each other at the same time.
	if c.LocalPeerID < peer.PeerID {
		go func() {
			conn, err := c.dialPeer(peer)
			if err != nil {
				slog.Error("Failed to connect to peer", "peer", peer.PeerID, "addr", peer.Addr, "err", err)
				return
			}

			// FIX: Wrap connection in connWrapper for per-connection write mutex
			wrapper := &connWrapper{conn: conn}

			c.mu.Lock()
			if oldWrapper, exists := c.connections[peer.PeerID]; exists && oldWrapper.conn != conn {
				oldWrapper.conn.Close()
			}
			c.connections[peer.PeerID] = wrapper
			c.mu.Unlock()

			slog.Info("Connected to peer", "peer", peer.PeerID, "addr", peer.Addr)
			// Send hello message so TCP knows who you are.
			// Note: Using SendMessage (not SendMessageToWrapper) for initial handshake
			// since we haven't stored the wrapper yet and want to fail fast.
			err = c.SendMessage(conn, TCPMessage{
				Type: PUBKEY_HANDSHAKE,
				// Send the public key on the handshake, so we can initiate encryption with the peer.
				Content:   c.EncryptionService.PublicKey.Bytes(),
				From:      c.LocalPeerID,
				Timestamp: time.Now(),
			})
			if err != nil {
				slog.Error("Failed to send hello message to peer", "peer", peer.PeerID, "err", err)
				conn.Close()
				c.mu.Lock()
				delete(c.connections, peer.PeerID)
				c.mu.Unlock()
				return
			}

			c.readLoop(peer.PeerID, wrapper)
		}()
		return nil
	}
	return nil
}
