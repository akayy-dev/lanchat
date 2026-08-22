package chat

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"
)

// used to differentiate between regular messages and other types (like exchange)
type TCPMessageType string

const (
	// Used to announce presence to listeners, and to exchange public keys for encryption.
	HELLO_MESSAGE TCPMessageType = "hello"
	CHAT_MESSAGE  TCPMessageType = "chat"
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
}

// dialPeer attempts to establish a TCP connection to the given peer.
func (c *ChatService) dialPeer(peer Peer) (net.Conn, error) {
	return net.DialTimeout("tcp", peer.Addr, 5*time.Second)
}

func (c *ChatService) Start(listener net.Listener) {
	c.listener = listener
	go c.acceptLoop()
}

// acceptLoop continuously accepts incoming TCP connections and handles them.
func (c *ChatService) acceptLoop() {
	for {
		conn, err := c.listener.Accept()
		if err != nil {
			// normal during service shutdown
			return
		}
		// TODO: Handle incoming connections
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

	if message.Type != HELLO_MESSAGE || message.From == "" {
		conn.Close()
		return
	}

	peerID := message.From
	slog.Info("received hello", "from", peerID)
	if peerID == c.LocalPeerID {
		conn.Close()
		return
	}

	c.readLoop(peerID, conn)
}

func (c *ChatService) readLoop(peerID string, conn net.Conn) {
	// when the connection is closed, remove it from the service
	defer conn.Close()
	defer delete(c.connections, peerID)

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
	slog.Info("Sent hello message")
	return nil
}

// Broadcast sends a chat message to all connected peers.
func (c *ChatService) Broadcast(content string) error {
	for peerID, conn := range c.connections {
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

func (c *ChatService) OnNewPeer(peer Peer) error {
	if c.LocalPeerID == peer.PeerID {
		// Ignore self
		return nil
	}

	// The peer with the lower ID will Dial the other,
	// to prevent both peers from trying to Dial each other at the same time.
	if c.LocalPeerID < peer.PeerID {
		// TODO: Dial logic here
		var err error
		go func() {
			c.connections[peer.PeerID], err = c.dialPeer(peer)
			if err != nil {
				slog.Error(fmt.Sprintf("Failed to connect to peer %s at %s", peer.PeerID, peer.Addr), slog.Any("err", err))
				return
			}
			slog.Info(fmt.Sprintf("Connected to peer %s at %s", peer.PeerID, peer.Addr))
			// send hello message so TCP knows how you are.
			c.SendMessage(c.connections[peer.PeerID], TCPMessage{
				Type:      HELLO_MESSAGE,
				From:      c.LocalPeerID,
				Timestamp: time.Now(),
			})
		}()
		return nil
	}
	return nil
}
