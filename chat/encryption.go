package chat

import (
	"crypto/ecdh"
	"crypto/rand"
)

type KeyRing struct {
	PublicKey  []byte
	PrivateKey []byte
	// SenderKeys are the public keys of other peers, key is the peerID
	SenderKeys map[string][]byte
}

func GenerateKeyPair() (*ecdh.PrivateKey, error) {
	curve := ecdh.X25519()
	return curve.GenerateKey(rand.Reader)
}
