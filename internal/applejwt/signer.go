// Package applejwt implements ES256 signing for Apple's distinct API audiences.
package applejwt

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Signer struct {
	key                                *ecdsa.PrivateKey
	keyID, issuerID, bundleID, subject string
	mu                                 sync.Mutex
	cached                             string
	expires                            time.Time
}

func ParseKey(encoded string) (*ecdsa.PrivateKey, error) {
	var der []byte
	if block, _ := pem.Decode([]byte(encoded)); block != nil {
		der = block.Bytes
	} else {
		var err error
		der, err = base64.StdEncoding.DecodeString(strings.Join(strings.Fields(encoded), ""))
		if err != nil {
			return nil, errors.New("invalid PKCS8 signing key")
		}
	}
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, err
	}
	ec, ok := key.(*ecdsa.PrivateKey)
	if !ok || ec.Curve != elliptic.P256() {
		return nil, errors.New("an ECDSA P-256 signing key is required")
	}
	return ec, nil
}

func New(encoded, keyID, issuerID, bundleID, subject string) (*Signer, error) {
	if keyID == "" || (issuerID == "" && subject == "") {
		return nil, errors.New("key ID and issuer ID (or individual-key subject) are required")
	}
	key, err := ParseKey(encoded)
	if err != nil {
		return nil, err
	}
	return &Signer{key: key, keyID: keyID, issuerID: issuerID, bundleID: bundleID, subject: subject}, nil
}

// Token uses a five-minute lifetime and refreshes before expiry. Connect tokens
// omit bid; individual Connect keys use sub instead of iss.
func (s *Signer) Token(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if s.cached != "" && now.Before(s.expires.Add(-30*time.Second)) {
		return s.cached, nil
	}
	claims := jwt.MapClaims{"aud": "appstoreconnect-v1", "iat": now.Unix(), "exp": now.Add(5 * time.Minute).Unix()}
	if s.issuerID != "" {
		claims["iss"] = s.issuerID
	}
	if s.bundleID != "" {
		claims["bid"] = s.bundleID
	}
	if s.subject != "" {
		claims["sub"] = s.subject
	}
	token, err := s.Sign(claims)
	if err != nil {
		return "", err
	}
	s.cached, s.expires = token, now.Add(5*time.Minute)
	return token, nil
}

func (s *Signer) Sign(claims jwt.MapClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = s.keyID
	token.Header["typ"] = "JWT"
	return token.SignedString(s.key)
}

func Nonce() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	data[6] = (data[6] & 0x0f) | 0x40
	data[8] = (data[8] & 0x3f) | 0x80
	s := hex.EncodeToString(data[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:], nil
}
