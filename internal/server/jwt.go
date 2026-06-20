package server

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var jwtSecret = []byte("well-ambient-jwt-secret-key-2026")

type JWTClaims struct {
	UserID      string   `json:"user_id"`
	Name        string   `json:"name"`
	WellOSToken string   `json:"wellos_token"`
	Avatar      string   `json:"avatar"`
	Department  string   `json:"department"`
	Groups      []string `json:"groups"`
	Permissions []string `json:"permissions"`
	Exp         int64    `json:"exp"`
}

// base64Encode encodes bytes to base64 URL safe string without padding
func base64Encode(src []byte) string {
	return base64.RawURLEncoding.EncodeToString(src)
}

// base64Decode decodes base64 URL safe string without padding
func base64Decode(src string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(src)
}

// GenerateJWT generates a HS256 signed JWT token valid for 2 hours
func GenerateJWT(userID, name, wellOSToken, avatar string, groups, permissions []string, department ...string) (string, error) {
	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}
	headerBytes, _ := json.Marshal(header)
	headerEncoded := base64Encode(headerBytes)

	dept := ""
	if len(department) > 0 {
		dept = department[0]
	}

	claims := JWTClaims{
		UserID:      userID,
		Name:        name,
		WellOSToken: wellOSToken,
		Avatar:      avatar,
		Department:  dept,
		Groups:      groups,
		Permissions: permissions,
		Exp:         time.Now().Add(2 * time.Hour).Unix(),
	}
	claimsBytes, _ := json.Marshal(claims)
	claimsEncoded := base64Encode(claimsBytes)

	signingInput := headerEncoded + "." + claimsEncoded

	h := hmac.New(sha256.New, jwtSecret)
	h.Write([]byte(signingInput))
	signature := base64Encode(h.Sum(nil))

	return signingInput + "." + signature, nil
}

// ParseJWT parses and validates a HS256 JWT token, returning the claims
func ParseJWT(tokenStr string) (*JWTClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	headerEncoded, claimsEncoded, signatureEncoded := parts[0], parts[1], parts[2]

	// Verify signature
	signingInput := headerEncoded + "." + claimsEncoded
	h := hmac.New(sha256.New, jwtSecret)
	h.Write([]byte(signingInput))
	expectedSignature := base64Encode(h.Sum(nil))

	if !hmac.Equal([]byte(signatureEncoded), []byte(expectedSignature)) {
		return nil, errors.New("invalid signature")
	}

	// Decode claims
	claimsBytes, err := base64Decode(claimsEncoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode claims: %v", err)
	}

	var claims JWTClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal claims: %v", err)
	}

	// Verify expiration
	if time.Now().Unix() > claims.Exp {
		return nil, errors.New("token expired")
	}

	return &claims, nil
}

// DeriveKeyAndIV derives a 32-byte key and 16-byte IV from passphrase and salt (OpenSSL compatible)
func DeriveKeyAndIV(passphrase []byte, salt []byte) (key []byte, iv []byte) {
	var combined []byte
	var prev []byte
	for len(combined) < 48 {
		h := md5.New()
		if len(prev) > 0 {
			h.Write(prev)
		}
		h.Write(passphrase)
		h.Write(salt)
		prev = h.Sum(nil)
		combined = append(combined, prev...)
	}
	return combined[:32], combined[32:48]
}

// DecryptCryptoJSAES decrypts standard CryptoJS AES ciphertext (OpenSSL format) with a passphrase
func DecryptCryptoJSAES(encryptedStr string, passphrase string) (string, error) {
	encryptedBytes, err := base64.StdEncoding.DecodeString(encryptedStr)
	if err != nil {
		// Try URL safe encoding just in case
		encryptedBytes, err = base64.RawURLEncoding.DecodeString(encryptedStr)
		if err != nil {
			return "", err
		}
	}

	if len(encryptedBytes) < 16 {
		return "", errors.New("ciphertext too short")
	}

	// CryptoJS formats output as: "Salted__" (8 bytes) + Salt (8 bytes) + actual ciphertext
	prefix := string(encryptedBytes[:8])
	if prefix != "Salted__" {
		return "", errors.New("invalid ciphertext prefix: missing 'Salted__'")
	}

	salt := encryptedBytes[8:16]
	ciphertext := encryptedBytes[16:]

	key, iv := DeriveKeyAndIV([]byte(passphrase), salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	if len(ciphertext)%aes.BlockSize != 0 {
		return "", errors.New("ciphertext block size is not a multiple of AES block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// PKCS7 Unpadding
	length := len(plaintext)
	if length == 0 {
		return "", errors.New("decrypted plaintext is empty")
	}
	unpadding := int(plaintext[length-1])
	if unpadding < 1 || unpadding > aes.BlockSize {
		return "", errors.New("invalid PKCS7 padding size")
	}
	for i := length - unpadding; i < length; i++ {
		if int(plaintext[i]) != unpadding {
			return "", errors.New("invalid PKCS7 padding byte")
		}
	}

	return string(plaintext[:length-unpadding]), nil
}
