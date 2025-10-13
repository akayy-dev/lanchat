package chat

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type MessageType int

const (
	MESSAGESEND = iota
	USERJOIN
	USERLEAVE
)


type User struct {
	Name     string `json:"username"`
	Hostname string `json:"hostname"`
}

type Message struct {
	Content string      `json:"message"`
	Type    MessageType `json:"type"`
	Time    time.Time   `json:"time"`
	User    User        `json:"user"`
}

func SendMessage(msg Message) {
	addr, err := net.ResolveUDPAddr("udp4", MULTICAST_ADDR)
	if err != nil {
		panic(err)
	}

	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		fmt.Printf("Connection error: %v\n", err)
		return
	}
	defer conn.Close()

	payload, err := json.Marshal(msg)
	if err != nil {
		panic(err)
	}

	_, err = conn.Write(payload)

	if err != nil {
		fmt.Printf("Send error: %v\n", err)
	}

}
