package util

import (
	"bytes"
	"crypto/aes"
	"encoding/base64"
	"errors"
)

// The original project (Tools.enAes/deAes) uses Java's default "AES" cipher,
// which is AES/ECB/PKCS5Padding, then Base64. We replicate it exactly so
// "remember me" cookies stay compatible with an existing deployment.

// EnAes encrypts data with AES-ECB + PKCS5 padding and returns Base64.
func EnAes(data, key string) (string, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	src := pkcs5Pad([]byte(data), block.BlockSize())
	dst := make([]byte, len(src))
	for i := 0; i < len(src); i += block.BlockSize() {
		block.Encrypt(dst[i:i+block.BlockSize()], src[i:i+block.BlockSize()])
	}
	return base64.StdEncoding.EncodeToString(dst), nil
}

// DeAes reverses EnAes.
func DeAes(data, key string) (string, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", err
	}
	if len(raw) == 0 || len(raw)%block.BlockSize() != 0 {
		return "", errors.New("invalid ciphertext length")
	}
	dst := make([]byte, len(raw))
	for i := 0; i < len(raw); i += block.BlockSize() {
		block.Decrypt(dst[i:i+block.BlockSize()], raw[i:i+block.BlockSize()])
	}
	out, err := pkcs5Unpad(dst, block.BlockSize())
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func pkcs5Pad(src []byte, blockSize int) []byte {
	padding := blockSize - len(src)%blockSize
	return append(src, bytes.Repeat([]byte{byte(padding)}, padding)...)
}

func pkcs5Unpad(src []byte, blockSize int) ([]byte, error) {
	n := len(src)
	if n == 0 {
		return nil, errors.New("empty")
	}
	padding := int(src[n-1])
	if padding == 0 || padding > blockSize || padding > n {
		return nil, errors.New("invalid padding")
	}
	return src[:n-padding], nil
}
