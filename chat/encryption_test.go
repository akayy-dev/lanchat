package chat

import (
	"crypto/ecdh"
	"crypto/rand"
	"testing"
)

func TestSenderKeyEncryptorRegisterPeer_InitializesSharedSecrets(t *testing.T) {
	enc := NewSenderKeyEncryptor()
	peerPubKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate peer key: %v", err)
	}

	if enc.SharedSecrets == nil {
		t.Fatal("SharedSecrets map should be initialized by constructor")
	}

	if _, err := enc.RegisterPeer("peer-1", peerPubKey.PublicKey().Bytes()); err != nil {
		t.Fatalf("RegisterPeer returned error: %v", err)
	}

	if _, ok := enc.SharedSecrets["peer-1"]; !ok {
		t.Fatal("shared secret was not stored for the peer")
	}
}
