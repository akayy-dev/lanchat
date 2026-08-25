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
	logFile, err := os.OpenFile("lanchat.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		panic(err)
	}
	defer logFile.Close()
	logger := slog.New(slog.NewTextHandler(io.MultiWriter(logFile, model), &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	// SETUP MULTICAST DISCOVERY NETWORKING
	peers := make(map[string]string)
	multicast := chat.NewMultiCastDiscovery()
	defer multicast.Close()
	discoveredPeerChan, discoveryErrChan, tcpListener, err := multicast.Listen()
	defer tcpListener.Close()
	if err != nil {
		panic(err)
	}

	// SETUP UNICAST CHAT SERVICE
	chatService := chat.NewChatService(multicast.PeerID)
	chatService.Start(tcpListener)
	defer chatService.Close()

	// Handle sending and receiving messages
	go func() {
		for chatService.ReceivedMessageChan != nil && model.SentMessageChan != nil {
			select {
			case message, ok := <-chatService.ReceivedMessageChan:
				if !ok {
					slog.Warn("ReceivedMessageChan closed, stopping message handling goroutine")
					chatService.ReceivedMessageChan = nil
					continue
				}
				slog.Debug("Received chat message")
				p.Send(ui.ReceivedChatMessage{Message: message})
			case message, ok := <-model.SentMessageChan:
				if !ok {
					slog.Warn("SentMessageChan closed, stopping message handling goroutine")
					model.SentMessageChan = nil
					continue
				}
				slog.Debug("Sending chat message")
				err := chatService.Broadcast(message.Content)
				if err != nil {
					panic(err)
				}
			}
		}
	}()

	// Goroutine for handling peer discovery its errors, and updating the UI accordingly.
	go func() {
		for discoveredPeerChan != nil || discoveryErrChan != nil {
			select {
			// On peer discovery
			case peer, ok := <-discoveredPeerChan:
				if !ok {
					discoveredPeerChan = nil
					continue
				}
				if _, ok := peers[peer.PeerID]; !ok {
					peers[peer.PeerID] = peer.Addr
					chatService.RegisterPeer(peer)
					p.Send(ui.NewUserMsg{Peer: peer})
				}
			// if there is an error in discovery, log it and continue
			case err, ok := <-discoveryErrChan:
				if !ok {
					discoveryErrChan = nil
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
