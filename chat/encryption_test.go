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

// Check if two byte slices are equal, useful for comparing keys
func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestHandshakeAndSharedSecret(t *testing.T) {
	alice := NewSenderKeyEncryptor()
	bob := NewSenderKeyEncryptor()

	// Alice registers Bob's public key
	if _, err := alice.RegisterPeer("bob", bob.PublicKey.Bytes()); err != nil {
		t.Fatalf("Alice failed to register Bob's public key: %v", err)
	}

	// Bob registers Alice's public key
	if _, err := bob.RegisterPeer("alice", alice.PublicKey.Bytes()); err != nil {
		t.Fatalf("Bob failed to register Alice's public key: %v", err)
	}

	// Both use each others public keys to compute the shared secret
	aliceSharedSecret, err := alice.ComputeSharedSecret(*bob.PublicKey)
	if err != nil {
		t.Fatalf("Alice failed to compute shared secret: %v", err)
	}

	bobSharedSecret, err := bob.ComputeSharedSecret(*alice.PublicKey)
	if err != nil {
		t.Fatalf("Bob failed to compute shared secret: %v", err)
	}

	t.Logf("Alice's shared secret: %x", aliceSharedSecret)
	t.Logf("Bob's shared secret: %x", bobSharedSecret)

	// Check that both shared secrets are equal
	if !equalBytes(aliceSharedSecret, bobSharedSecret) {
		t.Fatal("Shared secrets do not match")
	}

	// Create sender keys for each other, and decrypt on arrival
	aliceSenderKey := alice.DeriveSenderKey(aliceSharedSecret)
	bobSenderKey := bob.DeriveSenderKey(bobSharedSecret)
	alice.SenderKeys["bob"] = aliceSenderKey
	bob.SenderKeys["alice"] = bobSenderKey

	t.Logf("Alice -> Bob Sender Key: %x", aliceSenderKey)
	t.Logf("Bob -> Alice Sender Key: %x", bobSenderKey)

	if !equalBytes(aliceSenderKey, bobSenderKey) {
		t.Fatal("Sender keys do not match")
	}

	// test EncryptMessage by having two send encrypted messages and seeing if equal
	encryptedByAlice, err := alice.EncryptMessageWithSenderKey("bob", []byte("Hello Bob!"))
	if err != nil {
		t.Fatalf("Alice failed to encrypt message: %v", err)
	}

	encryptedByBob, err := bob.EncryptMessageWithSenderKey("alice", []byte("Hello Alice!"))
	if err != nil {
		t.Fatalf("Bob failed to encrypt message: %v", err)
	}

	t.Logf("Encrypted by Alice: %x", encryptedByAlice)
	t.Logf("Encrypted by Bob: %x", encryptedByBob)

	if equalBytes(encryptedByAlice, encryptedByBob) {
		t.Fatal("Encrypted messages should not be equal")
	}
}

func TestSenderKeyEncryptor_EncryptMessageWithSenderKey(t *testing.T) {
	alice := NewSenderKeyEncryptor()
	bob := NewSenderKeyEncryptor()

	if _, err := alice.RegisterPeer("bob", bob.PublicKey.Bytes()); err != nil {
		t.Fatalf("register bob public key: %v", err)
	}
	if _, err := bob.RegisterPeer("alice", alice.PublicKey.Bytes()); err != nil {
		t.Fatalf("register alice public key: %v", err)
	}

	aliceSharedSecret, err := alice.ComputeSharedSecret(*bob.PublicKey)
	if err != nil {
		t.Fatalf("compute alice shared secret: %v", err)
	}
	bobSharedSecret, err := bob.ComputeSharedSecret(*alice.PublicKey)
	if err != nil {
		t.Fatalf("compute bob shared secret: %v", err)
	}

	if !equalBytes(aliceSharedSecret, bobSharedSecret) {
		t.Fatal("shared secrets do not match")
	}

	alice.SenderKeys["bob"] = alice.DeriveSenderKey(aliceSharedSecret)
	bob.SenderKeys["alice"] = bob.DeriveSenderKey(bobSharedSecret)

	ciphertext1, err := alice.EncryptMessageWithSenderKey("bob", []byte("hello world"))
	if err != nil {
		t.Fatalf("encrypt from alice: %v", err)
	}
	ciphertext2, err := alice.EncryptMessageWithSenderKey("bob", []byte("hello world"))
	if err != nil {
		t.Fatalf("encrypt again from alice: %v", err)
	}

	if len(ciphertext1) == 0 || len(ciphertext2) == 0 {
		t.Fatal("ciphertext should not be empty")
	}
	if equalBytes(ciphertext1, ciphertext2) {
		t.Fatal("encrypting the same message twice should produce different ciphertexts because of a random nonce")
	}
	if equalBytes(ciphertext1, []byte("hello world")) {
		t.Fatal("ciphertext should be different from plaintext")
	}

	if _, err := (&SenderKeyEncryptor{SenderKeys: map[string][]byte{}}).EncryptMessageWithSenderKey("missing", []byte("hello")); err == nil {
		t.Fatal("encrypting without a peer sender key should return an error")
	}
}

func TestSenderKeyEncryptor_EncryptMessageWithSharedSecret(t *testing.T) {
	alice := NewSenderKeyEncryptor()
	bob := NewSenderKeyEncryptor()

	if _, err := alice.RegisterPeer("bob", bob.PublicKey.Bytes()); err != nil {
		t.Fatalf("register bob public key: %v", err)
	}
	if _, err := bob.RegisterPeer("alice", alice.PublicKey.Bytes()); err != nil {
		t.Fatalf("register alice public key: %v", err)
	}

	aliceSharedSecret, err := alice.ComputeSharedSecret(*bob.PublicKey)
	if err != nil {
		t.Fatalf("compute alice shared secret: %v", err)
	}
	bobSharedSecret, err := bob.ComputeSharedSecret(*alice.PublicKey)
	if err != nil {
		t.Fatalf("compute bob shared secret: %v", err)
	}

	alice.SharedSecrets["bob"] = aliceSharedSecret
	bob.SharedSecrets["alice"] = bobSharedSecret

	ciphertext1, err := alice.EncryptMessageWithSharedSecret("bob", []byte("encrypted via shared secret"))
	if err != nil {
		t.Fatalf("encrypt with shared secret: %v", err)
	}
	ciphertext2, err := alice.EncryptMessageWithSharedSecret("bob", []byte("encrypted via shared secret"))
	if err != nil {
		t.Fatalf("encrypt again with shared secret: %v", err)
	}

	if len(ciphertext1) == 0 || len(ciphertext2) == 0 {
		t.Fatal("ciphertext should not be empty")
	}
	if equalBytes(ciphertext1, ciphertext2) {
		t.Fatal("re-encrypting the same message should produce different ciphertext due to random nonce")
	}

	if _, err := (&SenderKeyEncryptor{SharedSecrets: map[string][]byte{}}).EncryptMessageWithSharedSecret("missing", []byte("hello")); err == nil {
		t.Fatal("encrypting without a shared secret should return an error")
	}
}
