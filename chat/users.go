package chat

import (
	"fmt"
	"math/rand"
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
	Color    string `json:"color"`
}

func randomHexString() string {
	rand.Seed(time.Now().UnixNano()) // Seed with current time
	r := rand.Intn(256)              // Red: 0–255
	g := rand.Intn(256)              // Green: 0–255
	b := rand.Intn(256)              // Blue: 0–255
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func CreateNewUser(username string, hostname string) User {
	return User{
		Name:     username,
		Hostname: hostname,
		Color:    randomHexString(),
	}
}
