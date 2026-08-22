package main

import (
	"LANChat/chat"
	"LANChat/ui"
	"io"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// SETUP UI
	model := ui.NewChatWindowModel()
	p := tea.NewProgram(model)

	// SETUP LOGGING
	logOutput := io.MultiWriter(os.Stderr, model)
	logger := slog.New(slog.NewTextHandler(logOutput, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	// SETUP NETWORKING
	peers := make(map[string]string)
	multicast := chat.NewMultiCastDiscovery()
	discoveredPeerChan, errChan, tcpListener, err := multicast.Listen()
	if err != nil {
		panic(err)
	}
	chatService := chat.NewChatService(multicast.PeerID)
	chatService.Start(tcpListener)
	defer tcpListener.Close()
	defer chatService.Close()
	defer multicast.Close()

	go func() {
		for discoveredPeerChan != nil || errChan != nil {
			select {
			case peer, ok := <-discoveredPeerChan:
				if !ok {
					discoveredPeerChan = nil
					continue
				}
				if _, ok := peers[peer.PeerID]; !ok {
					peers[peer.PeerID] = peer.Addr
					chatService.HandleNewPeer(peer)
					p.Send(ui.NewUserMsg{Peer: peer})
				}
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
