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
