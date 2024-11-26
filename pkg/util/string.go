package util

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

type SecureRandom struct {
	mu sync.Mutex
}

func NewSecureRandom() *SecureRandom {
	return &SecureRandom{}
}

func (sr *SecureRandom) GenerateHex(length int) (string, error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	bytes := make([]byte, (length+1)/2)
	
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	hexStr := hex.EncodeToString(bytes)

	if len(hexStr) > length {
		hexStr = hexStr[:length]
	}

	return hexStr, nil
}

func RandString(n int) string {
	sr := NewSecureRandom()
	hexStr, err := sr.GenerateHex(n)
	if err != nil {
		return "0000000000"
	}
	return hexStr
}

func BenchmarkGenerateHex(length int) string {
	sr := NewSecureRandom()
	hexStr, _ := sr.GenerateHex(length)
	return hexStr
}