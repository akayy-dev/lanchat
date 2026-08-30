package chat

import (
	"fmt"
	"math/rand/v2"
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
	r := rand.IntN(256) // Red: 0–255
	g := rand.IntN(256) // Green: 0–255
	b := rand.IntN(256) // Blue: 0–255
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func CreateNewUser(username string, hostname string) User {
	return User{
		Name:     username,
		Hostname: hostname,
		Color:    randomHexString(),
	}
}
