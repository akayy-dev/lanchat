package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
)

const (
	MULTICAST_ADDR = "224.0.0.1:9999"
)

type Message struct {
	Content string      `json:"message"`
	Type    MessageType `json:"type"`
	Time    time.Time   `json:"time"`
	User    User        `json:"user"`
	// Whether or not the message came from the client itself.
	Circular bool
}

type MessageService interface {
	Send(msg Message) error
	Listen() (<-chan Message, error)
	Close() error
}

type MulticastMessenger struct {
	conn        *net.UDPConn
	addr        *net.UDPAddr
	iface       *net.Interface
	listener    *net.UDPConn
	messageChan chan Message
	UserUUID    uuid.UUID
}

// Create a MulticastMesseneger
func NewMulticastMessenger() (*MulticastMessenger, error) {
	addr, err := net.ResolveUDPAddr("udp4", MULTICAST_ADDR)
	if err != nil {
		return nil, fmt.Errorf("resolve multicast addr: %v", err)
	}

	// Create socket to send messages
	sendConn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("dial udp: %w", err)
	}

	// Create socket to listen to messages
	listener, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		sendConn.Close()
		return nil, fmt.Errorf("multicast listener: %w", err)
	}

	listener.SetReadBuffer(2048)
	return &MulticastMessenger{
		conn:     sendConn,
		addr:     addr,
		listener: listener,
	}, nil
}

func (m *MulticastMessenger) Send(msg Message) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("error marshalling JSON message %w", err)
	}

	// Don't send messages meant for the client to the network.
	if msg.Type != CLIENT {
		_, err = m.conn.Write(payload)
	} else {
		if m.messageChan != nil {
			select {
			case m.messageChan <- msg:
			default:
			}
		}
	}
	return err
}

func (m *MulticastMessenger) Listen() (<-chan Message, error) {
	if m.messageChan == nil {
		m.messageChan = make(chan Message, 32)
	}

	go func() {
		defer close(m.messageChan)
		buffer := make([]byte, 2048)
		for {
			n, _, err := m.listener.ReadFromUDP(buffer)
			if err != nil {
				// Known error that occurs when leaving the chat.
				if errors.Is(err, net.ErrClosed) {
					continue
				}
				fmt.Printf("read error %v\n", err)
				continue
			}

			var msg Message
			if err := json.Unmarshal(buffer[:n], &msg); err != nil {
				fmt.Printf("json unmarshal error %v\n", err)
				continue
			}
			if msg.User.UUID == m.UserUUID {
				msg.Circular = true
			}
			m.messageChan <- msg
		}
	}()

	return m.messageChan, nil
}

func (m *MulticastMessenger) Close() error {
	if m.conn != nil {
		m.conn.Close()
	}
	if m.listener != nil {
		m.listener.Close()
	}

	return nil
}
