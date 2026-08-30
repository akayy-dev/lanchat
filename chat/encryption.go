package chat

import (
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"log/slog"

	"golang.org/x/crypto/hkdf"
)

type Encryptor interface {
	RegisterPeer(peerID string, publicKey []byte) error
	DeregisterPeer(peerID string) error
	Encrypt(message []byte, peerID string) ([]byte, error)
	Decrypt(encryptedMessage []byte, peerID string) ([]byte, error)
}

// Create a new SenderKeyEncryptor
func NewSenderKeyEncryptor() *SenderKeyEncryptor {
	curve := ecdh.X25519()
	privateKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		panic(err) // TODO: Handle error appropriately in production code
	}
	publicKey := privateKey.PublicKey()

	return &SenderKeyEncryptor{
		PublicKey:     publicKey,
		PrivateKey:    privateKey,
		SenderKeys:    make(map[string][]byte),
		SharedSecrets: make(map[string][]byte),
	}
}

type SenderKeyEncryptor struct {
	PublicKey  *ecdh.PublicKey
	PrivateKey *ecdh.PrivateKey
	// SenderKeys are the public keys of other peers, key is the peerID
	SenderKeys    map[string][]byte
	SharedSecrets map[string][]byte // Store shared secrets for each peer
}

func (e *SenderKeyEncryptor) ComputeSharedSecret(peerPublicKey ecdh.PublicKey) ([]byte, error) {
	sharedSecret, err := e.PrivateKey.ECDH(&peerPublicKey)
	if err != nil {
		return nil, err
	}
	return sharedSecret, nil
}

func (e *SenderKeyEncryptor) DeriveSenderKey(sharedSecret []byte) []byte {
	h := hkdf.New(sha256.New, sharedSecret, nil, []byte("lanchat-sender-key"))
	key := make([]byte, 32) // 256-bit key
	_, err := h.Read(key)
	if err != nil {
		panic(err) // TODO: Handle error appropriately in production code
	}
	return key
}

// RegisterPeer registers a peer's public key and returns the shared secret for that peer. It computes the shared secret using ECDH and stores it in the SenderKeys map.
func (e *SenderKeyEncryptor) RegisterPeer(peerID string, publicKey []byte) ([]byte, error) {
	peerPubKey, err := ecdh.X25519().NewPublicKey(publicKey)
	if err != nil {
		return nil, err
	}
	sharedSecret, err := e.ComputeSharedSecret(*peerPubKey)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to compute shared secret for %s", peerID), slog.String("error", err.Error()))
		return nil, err
	}

	slog.Debug(fmt.Sprintf("Computing shared secret for %s", peerID), slog.String("secret", string(sharedSecret)))
	e.SharedSecrets[peerID] = sharedSecret
	return sharedSecret, nil
}
