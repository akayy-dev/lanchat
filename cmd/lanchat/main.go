package main

import (
	"LANChat/chat"
	"fmt"
)

func main() {
	peers := make(map[string]string)
	multicast := chat.MultiCastDiscovery{}
	peerChan, err := multicast.Listen()
	if err != nil {
		panic(err)
	}

	for peer := range peerChan {
		_, ok := peers[peer.PeerID]
		if !ok {
			fmt.Printf("Peer discovered - %s:%s\n", peer.PeerID, peer.Addr)
			peers[peer.PeerID] = peer.Addr
		}
	}
}
