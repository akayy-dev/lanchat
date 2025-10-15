package encryption

import (
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/nacl/box"
)

func EncryptGroupKey(groupKey [32]byte, peerPub *[32]byte, priv *[32]byte) []byte {
	// implement encryption logic here
	var nonce [24]byte
	rand.Read(nonce[:])

	ciphertext := box.Seal(nonce[:], groupKey[:], &nonce, peerPub, priv)
	return ciphertext
}

func GenerateGroupKey() (groupkey [32]byte) {
	rand.Read(groupkey[:])
	return groupkey
}

func DecryptGroupKey(ciphertext []byte, nonce *[24]byte, peerPub *[32]byte, priv *[32]byte) ([32]byte, error) {
	var groupKey [32]byte
	copy(nonce[:], ciphertext[:24])

	plaintext, ok := box.Open(nil, ciphertext[24:], nonce, peerPub, priv)
	if !ok {
		return groupKey, fmt.Errorf("decryption failed, invalid key or data.")
	}

	copy(groupKey[:], plaintext)
	return groupKey, nil
}
