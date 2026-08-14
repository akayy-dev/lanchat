package chat

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/user"

	"github.com/schollz/peerdiscovery"
)

type Peer struct {
	PeerID string `json:"peer_id"`
	Addr   string `json:"addr"`
}

// Get the IP address recipients are supposed to communicate with over unicast
func getOutboundIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP, nil
}


func buildAnnouncement(peerID string) (Peer, net.Listener, error) {
	// TODO: Find out how listener shold be used in the return value
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return Peer{}, nil, err
	}

	ip, err := getOutboundIP()
	if err != nil {
		listener.Close()
		return Peer{}, nil, err
	}

	port := listener.Addr().(*net.TCPAddr).Port
	addr := fmt.Sprintf("%s:%d", ip.String(), port)

	return Peer{PeerID: peerID, Addr: addr}, listener, nil
}

type MultiCastDiscovery struct {
	discovery *peerdiscovery.PeerDiscovery
}

func (m *MultiCastDiscovery) Listen() ( <-chan Peer, error ) {
	user, err := user.Current()
	if err != nil {
		return nil, err
	}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	peerID := user.Username + "@" + hostname
	announcement, _, err := buildAnnouncement(peerID)
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(announcement)
	if err != nil {
		return nil, err
	}

	peerChan := make(chan Peer, 16)
	discoverySettings := peerdiscovery.Settings{
		Payload:   payload,
		Limit:     -1,
		AllowSelf: false,
		TimeLimit: -1,
		Notify: func(d peerdiscovery.Discovered) {
			var p Peer
			if err := json.Unmarshal(d.Payload, &p); err != nil {
				return // skip invalid packets
			}
			peerChan <- p
		},
	}
	go func() {
		m.discovery, err = peerdiscovery.NewPeerDiscovery(discoverySettings)
		if err != nil {
			panic(err)
		}
	}()

	return peerChan, err
}

func (m *MultiCastDiscovery) Close() error {
	if (m.discovery == nil) {
		return fmt.Errorf("m.discovery is nil")
	} else {
		// I think this closes the listener, idk?
		m.discovery.Shutdown()
		return nil
	}
}
