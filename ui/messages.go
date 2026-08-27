package ui

import (
	"LANChat/chat"
	"time"
)

type ChatMessageType int

const (
	SYSTEM_MESSAGE ChatMessageType = iota
	USER_MESSAGE
	USER_JOIN
	USER_LEFT
)

type ChatMessage struct {
	Type    ChatMessageType
	Content string
	From    string
}

type NewUserMsg struct {
	Peer chat.Peer
}

// FIX: Decoupled ReceivedChatMessage from chat.TCPMessage.
// Previously this struct embedded chat.TCPMessage directly, coupling UI to network layer.
// Now it contains only display-relevant fields (From, Content, Timestamp).
// This allows the network layer to add encryption fields without affecting UI code.
// The conversion from TCPMessage to this type happens in main.go.
type ReceivedChatMessage struct {
	From      string
	Content   string
	Timestamp time.Time
}

type LeftUserMsg struct {
	Peer chat.Peer
}

func NewMessageUpdate(msgType ChatMessageType, content string, from string) func() MessageUpdate {
	return func() MessageUpdate {
		return MessageUpdate{
			Type:    msgType,
			Content: content,
			From:    from,
		}
	}
}

type MessageUpdate struct {
	Type    ChatMessageType
	Content string
	From    string
}
