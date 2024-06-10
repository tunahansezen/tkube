package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"embed"
)

var (
	//go:embed resources
	f        embed.FS
	encBytes = []byte{35, 46, 57, 24, 85, 35, 24, 74, 87, 35, 88, 98, 66, 32, 14, 05}
)

func EncryptFile(plainText []byte) ([]byte, error) {
	key, err := f.ReadFile("resources/aes.key")
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	cfb := cipher.NewCFBEncrypter(block, encBytes)
	cipherText := make([]byte, len(plainText))
	cfb.XORKeyStream(cipherText, plainText)
	return cipherText, nil
}

func DecryptFile(cipherText []byte) ([]byte, error) {
	key, err := f.ReadFile("resources/aes.key")
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	cfb := cipher.NewCFBDecrypter(block, encBytes)
	plainText := make([]byte, len(cipherText))
	cfb.XORKeyStream(plainText, cipherText)
	return plainText, nil
}
