package main

import (
	"LANChat/chat"
	"fmt"
)

func main() {
	peers := make(map[string]string)
	multicast := chat.MultiCastDiscovery{}
	peerChan, errChan, tcpListener, err := multicast.Listen()
	if err != nil {
		panic(err)
	}
	defer tcpListener.Close()
	defer multicast.Close()

	// Keep the TCP port announced through multicast open for incoming messages.
	chat.StartUnicastListener(tcpListener, func(message []byte) {
		fmt.Printf("Message received - %s\n", message)
	})

	for peerChan != nil || errChan != nil {
		select {
		case peer, ok := <-peerChan:
			if !ok {
				peerChan = nil
				continue
			}
			if _, ok := peers[peer.PeerID]; !ok {
				fmt.Printf("Peer discovered - %s:%s\n", peer.PeerID, peer.Addr)
				peers[peer.PeerID] = peer.Addr
			}
		case err, ok := <-errChan:
			if !ok {
				errChan = nil
				continue
			}
			panic(err)
		}
	}
}
