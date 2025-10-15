package chat

import (
	cryptoRand "crypto/rand"
	"fmt"
	mathRand "math/rand"

	"github.com/google/uuid"
	"golang.org/x/crypto/nacl/box"
)

type MessageType int

const (
	// User sends a message
	MESSAGESEND = iota
	// User joins the room
	USERJOIN
	// User leaves the room
	USERLEAVE
	// User sends key
	KEYSEND
	// Messages only for the client, NOT to be sent over the network.
	CLIENT
	// Request the group key
	KEYREQUEST
)

type KeyChain struct {
	PublicKey  *[32]byte `json:"public_key"`
	PrivateKey *[32]byte `json:"-"`
	GroupKey   *[32]byte `json:"-"`
}

type User struct {
	Name     string    `json:"username"`
	Hostname string    `json:"hostname"`
	Color    string    `json:"color"`
	UUID     uuid.UUID `json:"uuid"`
	Keys     KeyChain  `json:"keys"`
}

func randomHexString() string {
	r := mathRand.Intn(256) // Red: 0–255
	g := mathRand.Intn(256) // Green: 0–255
	b := mathRand.Intn(256) // Blue: 0–255
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func CreateNewUser(username string, hostname string) User {
	pub, priv, err := box.GenerateKey(cryptoRand.Reader)
	if err != nil {
		panic(err)
	}
	return User{
		Name:     username,
		Hostname: hostname,
		Color:    randomHexString(),
		UUID:     uuid.New(),
		Keys: KeyChain{
			PublicKey:  pub,
			PrivateKey: priv,
		},
	}
}
