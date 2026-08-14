package chat

type KeyRing struct {
	PublicKey  []byte
	PrivateKey []byte
	// SenderKeys are the public keys of other peers, key is the peerID
	SenderKeys map[string][]byte
}
