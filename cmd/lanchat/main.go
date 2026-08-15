package main

import (
	"LANChat/chat"
	"LANChat/ui"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	peers := make(map[string]string)
	multicast := chat.NewMultiCastDiscovery()
	discoveredPeerChan, lostPeerChan, errChan, tcpListener, err := multicast.Listen()
	if err != nil {
		panic(err)
	}
	defer tcpListener.Close()
	defer multicast.Close()

	// Keep the TCP port announced through multicast open for incoming messages.
	chat.StartUnicastListener(tcpListener, func(message []byte) {
		fmt.Printf("Message received - %s\n", message)
	})

	model := ui.NewChatWindowModel()
	p := tea.NewProgram(model)

	go func() {
		for discoveredPeerChan != nil || lostPeerChan != nil || errChan != nil {
			select {
			case peer, ok := <-discoveredPeerChan:
				if !ok {
					discoveredPeerChan = nil
					continue
				}
				if _, ok := peers[peer.PeerID]; !ok {
					// fmt.Printf("Peer discovered - %s:%s\n", peer.PeerID, peer.Addr)
					peers[peer.PeerID] = peer.Addr
					p.Send(ui.NewUserMsg{Peer: peer})
				}
			case peer, ok := <-lostPeerChan:
				if !ok {
					lostPeerChan = nil
					continue
				}
				delete(peers, peer.PeerID)
				p.Send(ui.LeftUserMsg{Peer: peer})
			case err, ok := <-errChan:
				if !ok {
					errChan = nil
					continue
				}
				panic(err)
			}
		}
	}()
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
