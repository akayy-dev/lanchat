package ui

type ChatMessage int
const (
	SYSTEM_MESSAGE ChatMessage = iota
	USER_MESSAGE
)

type Message struct {
	Type ChatMessage
}
