package chat

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"sync"

	"golang.org/x/crypto/hkdf"
)

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
	// SenderKeys are the derived symmetric keys for encrypting messages to each peer
	SenderKeys    map[string][]byte
	SharedSecrets map[string][]byte // Store shared secrets for each peer
	mu            sync.RWMutex      // Protects SenderKeys and SharedSecrets maps
}

func (e *SenderKeyEncryptor) EncryptMessageWithSenderKey(peerID string, message []byte) ([]byte, error) {
	e.mu.RLock()
	senderKey, exists := e.SenderKeys[peerID]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no sender key found for peer %s", peerID)
	}

	block, _ := aes.NewCipher(senderKey)
	gcm, _ := cipher.NewGCM(block)

	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce) //refresh the nonce every function call to ensure randomness
	return gcm.Seal(nonce, nonce, message, nil), nil
}

func (e *SenderKeyEncryptor) EncryptMessageWithSharedSecret(peerID string, message []byte) ([]byte, error) {
	e.mu.RLock()
	sharedSecret, exists := e.SharedSecrets[peerID]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no shared secret found for peer %s", peerID)
	}

	block, _ := aes.NewCipher(sharedSecret)
	gcm, _ := cipher.NewGCM(block)

	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce) //refresh the nonce every function call to ensure randomness
	return gcm.Seal(nonce, nonce, message, nil), nil
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

// SetSenderKey stores a sender key for a peer in a thread-safe manner.
func (e *SenderKeyEncryptor) SetSenderKey(peerID string, key []byte) {
	e.mu.Lock()
	e.SenderKeys[peerID] = key
	e.mu.Unlock()
}

// GetSharedSecret retrieves a shared secret for a peer in a thread-safe manner.
// Returns the secret and a boolean indicating if it exists.
func (e *SenderKeyEncryptor) GetSharedSecret(peerID string) ([]byte, bool) {
	e.mu.RLock()
	secret, exists := e.SharedSecrets[peerID]
	e.mu.RUnlock()
	return secret, exists
}

// RegisterPeer registers a peer's public key and returns the shared secret for that peer.
// It computes the shared secret using ECDH and stores it in the SharedSecrets map.
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

	e.mu.Lock()
	e.SharedSecrets[peerID] = sharedSecret
	e.mu.Unlock()

	return sharedSecret, nil
}
