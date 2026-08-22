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

// used to differentiate between regular messages and other types (like exchange)
type TCPMessageType string

const (
	// Used to announce presence to listeners, and to exchange public keys for encryption.
	HELLO_MESSAGE  TCPMessageType = "hello"
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
		LocalPeerID: localPeerID,
		connections: make(map[string]net.Conn),
		peers:       make(map[string]Peer),
	}
}

type ChatService struct {
	LocalPeerID string
	connections map[string]net.Conn
	listener    net.Listener
	peers       map[string]Peer
	mu          sync.RWMutex
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
}

// Close gracefully shuts down the ChatService, closing all active connections and the listener.
func (c *ChatService) Close() {
	c.mu.Lock()
	// snapshot of active connections
	conns := make([]net.Conn, 0, len(c.connections))
	for peerID, conn := range c.connections {
		conns = append(conns, conn)
		delete(c.connections, peerID)
	}
	listener := c.listener
	c.listener = nil
	c.mu.Unlock()

	// Close all active connections
	for _, conn := range conns {
		conn.Close()
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

// handleIncomingConnection reads a message from the connection and processes it based on its type.
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

	switch message.Type {
	case HELLO_MESSAGE:
		slog.Info("received hello", "from", peerID)
	case CHAT_MESSAGE:
		slog.Info("Received chat message", "from", message.From, "content", string(message.Content))
	default:
		slog.Warn("Unknown message type received", "type", message.Type)
		conn.Close()
		return
	}

	c.mu.Lock()
	if oldConn, exists := c.connections[peerID]; exists && oldConn != conn {
		oldConn.Close()
	}
	c.connections[peerID] = conn // Store the connection for future communication
	c.mu.Unlock()
	c.readLoop(peerID, conn)
}

func (c *ChatService) readLoop(peerID string, conn net.Conn) {
	// when the connection is closed, remove it from the service
	defer conn.Close()
	defer func() {
		c.mu.Lock()
		if existingConn, ok := c.connections[peerID]; ok && existingConn == conn {
			delete(c.connections, peerID)
		}
		c.mu.Unlock()
	}()

	for {
		message, err := c.readMessage(conn)
		if err != nil {
			return
		}
		slog.Info(
			"received message",
			"from", message.From,
			"content", string(message.Content),
		)
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

// Broadcast sends a chat message to all connected peers.
func (c *ChatService) Broadcast(content string) error {
	c.mu.RLock()
	// Build snapshot
	snapshot := make(map[string]net.Conn, len(c.connections))
	for peerID, conn := range c.connections {
		snapshot[peerID] = conn
	}
	c.mu.RUnlock()

	for peerID, conn := range snapshot {
		if peerID == c.LocalPeerID {
			continue // Skip sending to self
		}
		err := c.SendMessage(conn, TCPMessage{
			Type:      CHAT_MESSAGE,
			From:      c.LocalPeerID,
			Content:   []byte(content),
			Timestamp: time.Now(),
		})
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to send message to peer %s", peerID), slog.Any("err", err))
		}
	}
	return nil
}

func (c *ChatService) HandleNewPeer(peer Peer) error {
	if c.LocalPeerID == peer.PeerID {
		// Ignore self
		return nil
	}

	// The peer with the lower ID will Dial the other,
	// to prevent both peers from trying to Dial each other at the same time.
	if c.LocalPeerID < peer.PeerID {
		// TODO: Dial logic here
		go func() {
			conn, err := c.dialPeer(peer)
			if err != nil {
				slog.Error(fmt.Sprintf("Failed to connect to peer %s at %s", peer.PeerID, peer.Addr), slog.Any("err", err))
				return
			}

			c.mu.Lock()
			if oldConn, exists := c.connections[peer.PeerID]; exists && oldConn != conn {
				oldConn.Close()
			}
			c.connections[peer.PeerID] = conn
			c.mu.Unlock()

			slog.Info(fmt.Sprintf("Connected to peer %s at %s", peer.PeerID, peer.Addr))
			// send hello message so TCP knows how you are.
			err = c.SendMessage(conn, TCPMessage{
				Type:      HELLO_MESSAGE,
				From:      c.LocalPeerID,
				Timestamp: time.Now(),
			})
			if err != nil {
				slog.Error(fmt.Sprintf("Failed to send hello message to peer %s", peer.PeerID), slog.Any("err", err))
				conn.Close()
				c.mu.Lock()
				delete(c.connections, peer.PeerID)
				c.mu.Unlock()
				return
			}

			c.readLoop(peer.PeerID, conn)
		}()
		return nil
	}
	return nil
}
