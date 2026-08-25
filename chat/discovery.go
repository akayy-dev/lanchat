package chat

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/user"
	"strconv"
	"sync"
	"time"

	"github.com/schollz/peerdiscovery"
)

const (
	LastSeenTimeout = 10 * time.Second
)

type Peer struct {
	PeerID string `json:"peer_id"`
	// Addr is derived locally from the multicast packet's source address. It is
	// intentionally not broadcast because a host can have multiple interfaces.
	Addr string `json:"-"`
	// Port is the TCP port on which this peer accepts unicast connections.
	Port int `json:"port"`
}

func createPeer(peerID string) (Peer, net.Listener, error) {
	// Binding to :0 reserves an available TCP port before we announce it.
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return Peer{}, nil, err
	}

	port := listener.Addr().(*net.TCPAddr).Port
	return Peer{PeerID: peerID, Port: port}, listener, nil
}

type MultiCastDiscovery struct {
	PeerID   string
	mu       sync.Mutex
	stopChan chan struct{}
	stopOnce sync.Once
	// LastSeen tracks the last time we saw each peer, if a peer hasn't been seen
	// in a while, we can assume they're offline.
	LastSeen map[string]time.Time
}

func NewMultiCastDiscovery() *MultiCastDiscovery {
	return &MultiCastDiscovery{
		LastSeen: make(map[string]time.Time),
	}
}

// Listen starts multicast discovery and immediately returns the discovered-peer
// stream, asynchronous error stream, and the TCP listener advertised to peers.
// The caller owns tcpListener and must close it when shutting down.
func (m *MultiCastDiscovery) Listen() (<-chan Peer, <-chan error, net.Listener, error) {
	user, err := user.Current()
	if err != nil {
		return nil, nil, nil, err
	}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, nil, nil, err
	}

	m.PeerID = user.Username + "@" + hostname
	announcement, tcpListener, err := createPeer(m.PeerID)
	if err != nil {
		return nil, nil, nil, err
	}

	payload, err := json.Marshal(announcement)
	if err != nil {
		tcpListener.Close()
		return nil, nil, nil, err
	}

	// Check if discovery has already been started.
	m.mu.Lock()
	if m.stopChan != nil {
		m.mu.Unlock()
		tcpListener.Close()
		return nil, nil, nil, fmt.Errorf("multicast discovery has already been started")
	}

	// peerdiscovery checks StopChan between discovery iterations. Storing it
	// before the goroutine starts makes Close safe immediately after Listen.
	m.stopChan = make(chan struct{})
	stopChan := m.stopChan
	m.mu.Unlock()

	discoveredPeerChan := make(chan Peer, 16)
	errChan := make(chan error, 1)
	discoverySettings := peerdiscovery.Settings{
		Payload:   payload,
		Limit:     -1,
		AllowSelf: false,
		TimeLimit: -1,
		StopChan:  stopChan,
		Notify: func(d peerdiscovery.Discovered) {
			var p Peer
			if err := json.Unmarshal(d.Payload, &p); err != nil {
				return // skip invalid packets
			}
			if p.Port < 1 || p.Port > 65535 {
				return // skip announcements without a usable TCP port
			}

			// d.Address is the sender IP as observed on this LAN, which is more
			// reliable than an IP selected by the sender for another destination.
			p.Addr = net.JoinHostPort(d.Address, strconv.Itoa(p.Port))

			// Do not leave the discovery callback blocked during shutdown if the
			// consumer has stopped reading discovered peers.
			select {
			case discoveredPeerChan <- p:
			case <-stopChan:
			}
		},
	}
	go func() {
		defer close(discoveredPeerChan)
		defer close(errChan)
		defer func() {
			// Some network setup failures in third-party discovery implementations
			// can surface as panics; expose them through the error stream instead.
			if recovered := recover(); recovered != nil {
				errChan <- fmt.Errorf("multicast discovery failed: %v", recovered)
			}
		}()

		_, err = peerdiscovery.NewPeerDiscovery(discoverySettings)
		if err != nil {
			// Errors occur after Listen has returned, so report them rather than
			// panicking in a background goroutine.
			errChan <- err
		}
	}()

	return discoveredPeerChan, errChan, tcpListener, nil
}

// Close asks the blocking peerdiscovery loop to exit. It does not close the
// TCP listener returned by Listen because that listener is owned by the caller.
func (m *MultiCastDiscovery) Close() error {
	m.mu.Lock()
	stopChan := m.stopChan
	m.mu.Unlock()

	if stopChan == nil {
		return fmt.Errorf("multicast discovery has not been started")
	}

	m.stopOnce.Do(func() { close(stopChan) })
	return nil
}
