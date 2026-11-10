package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func NewSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成SessionID失败 %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func main() {
	str, _ := NewSessionID()
	fmt.Println(str)
}
