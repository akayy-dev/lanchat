package main

import (
	"LANChat/chat"
	"LANChat/ui"
	"context"
	"io"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// Use this to allow the ChatWindowModel to implement the io.Writer interface, enabling it to be used as a log writer.
type TeaLogWriter struct {
	Program *tea.Program
}

// Writes a SYSTEM_MESSAGE to the screen as if it was a chat message, useful for logging as it implementst he io.Writer interface.
func (w TeaLogWriter) Write(b []byte) (int, error) {
	w.Program.Send(ui.MessageUpdate{
		Type:    ui.SYSTEM_MESSAGE,
		Content: string(b),
	})
	return len(b), nil
}

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
	logger := slog.New(slog.NewTextHandler(io.MultiWriter(logFile, TeaLogWriter{Program: p}), &slog.HandlerOptions{Level: slog.LevelDebug}))
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

	ctx, cancel := context.WithCancel(context.Background())
	// shutdown goroutine when main() ends
	defer cancel()

	// Handle receiving messages from peers and forwarding to UI.
	go func() {
		for {
			select {
			// wait for context cancellation to gracefully shut down the goroutine
			case <-ctx.Done():
				slog.Debug("Receive message goroutine shutting down")
				return
			case message, ok := <-chatService.ReceivedMessageChan:
				if !ok {
					slog.Warn("ReceivedMessageChan closed")
					return
				}
				slog.Debug("Received chat message", "from", message.From)
				p.Send(ui.ReceivedChatMessage{
					From:      message.From,
					Content:   string(message.Content),
					Timestamp: message.Timestamp,
				})
			}
		}
	}()

	// GOROUTINE: Handle sending messages from UI to network layer.
	// FIX: Separated from receive goroutine for cleaner code and independent lifecycle.
	go func() {
		for {
			select {
			case <-ctx.Done():
				slog.Debug("Send message goroutine shutting down")
				return
			case message, ok := <-model.SentMessageChan:
				if !ok {
					slog.Warn("SentMessageChan closed")
					return
				}
				slog.Debug("Sending chat message")
				// Note: Broadcast errors are now sent to ErrorChan instead of returned here
				chatService.Broadcast(message.Content)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				slog.Debug("Error handler goroutine shutting down")
				return
			case err, ok := <-chatService.ErrorChan:
				if !ok {
					slog.Debug("ErrorChan closed")
					return
				}
				// Display network errors to the user via the UI
				slog.Error("Network error", "err", err)
				p.Send(ui.MessageUpdate{
					Type:    ui.SYSTEM_MESSAGE,
					Content: "Network error: " + err.Error(),
				})
			}
		}
	}()

	// GOROUTINE: Handle peer discovery and its errors, updating the UI accordingly.
	// FIX: Uses context for clean shutdown instead of nil-assignment pattern.
	go func() {
		for {
			select {
			case <-ctx.Done():
				slog.Debug("Peer discovery goroutine shutting down")
				return
			case peer, ok := <-discoveredPeerChan:
				if !ok {
					slog.Debug("discoveredPeerChan closed")
					return
				}
				if _, ok := peers[peer.PeerID]; !ok {
					peers[peer.PeerID] = peer.Addr
					chatService.RegisterPeer(peer)
					p.Send(ui.NewUserMsg{Peer: peer})
				}
			case err, ok := <-discoveryErrChan:
				if !ok {
					slog.Debug("discoveryErrChan closed")
					return
				}
				slog.Error("Discovery error", "err", err)
				p.Send(ui.MessageUpdate{
					Type:    ui.SYSTEM_MESSAGE,
					Content: "Discovery error: " + err.Error(),
				})
			}
		}
	}()
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
