package discovery

import (
	"encoding/json"
	"os"
	"os/user"

	"github.com/schollz/peerdiscovery"
)

const (
	MULTICAST_ADDR = "239.255.42.99:9999"
)

type Announcement struct {
	PeerID string `json:"peer_id"`
	Addr   string `json:"addr"`
}

func StartListening(onFound func(Announcement)) (*peerdiscovery.PeerDiscovery, error) {
	user, err := user.Current()
	if err != nil {
		return nil, err
	}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	peerID := user.Username + "@" + hostname
	announcement := Announcement{
		PeerID: peerID,
		Addr:   "",
	}

	payload, err := json.Marshal(announcement)
	if err != nil {
		return nil, err
	}
	discoverySettings := peerdiscovery.Settings{
		Payload:   payload,
		Limit:     -1,
		AllowSelf: false,
		TimeLimit: -1,
		Notify: func(d peerdiscovery.Discovered) {
			var a Announcement
			if err := json.Unmarshal(d.Payload, &a); err != nil {
				return // skip invalid packets
			}
			onFound(a)
		},
	}

	discovery, err := peerdiscovery.NewPeerDiscovery(discoverySettings)
	if err != nil {
		return nil, err
	}

	return discovery, err
}
