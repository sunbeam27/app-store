// Package offers creates Apple's promotional and introductory-offer signatures.
package offers

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sunbeam27/app-store/internal/applejwt"
)

type PromotionalOfferSignatureCreator struct {
	key             *ecdsa.PrivateKey
	keyID, bundleID string
}

func NewPromotionalOfferSignatureCreator(signingKey, keyID, bundleID string) (*PromotionalOfferSignatureCreator, error) {
	if keyID == "" || bundleID == "" {
		return nil, errors.New("key ID and bundle ID are required")
	}
	key, err := applejwt.ParseKey(signingKey)
	if err != nil {
		return nil, err
	}
	return &PromotionalOfferSignatureCreator{key: key, keyID: keyID, bundleID: bundleID}, nil
}

// CreateSignature produces the legacy Base64 DER ECDSA signature. Timestamp is
// in milliseconds; nonce must be a newly generated UUID for each signature.
func (s *PromotionalOfferSignatureCreator) CreateSignature(productID, offerID, appAccountToken, nonce string, timestamp int64) (string, error) {
	if productID == "" || offerID == "" {
		return "", errors.New("product ID and offer identifier are required")
	}
	if !validUUID(nonce) {
		return "", errors.New("nonce must be a UUID")
	}
	payload := strings.Join([]string{s.bundleID, s.keyID, productID, offerID, strings.ToLower(appAccountToken), strings.ToLower(nonce), strconv.FormatInt(timestamp, 10)}, "\u2063")
	digest := sha256.Sum256([]byte(payload))
	signature, err := ecdsa.SignASN1(rand.Reader, s.key, digest[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func validUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, c := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

type SignatureConfig struct{ SigningKey, KeyID, IssuerID, BundleID string }
type JWSSignatureCreator struct {
	signer                       *applejwt.Signer
	audience, issuerID, bundleID string
}

func NewJWSSignatureCreator(audience string, config SignatureConfig) (*JWSSignatureCreator, error) {
	if audience == "" || config.BundleID == "" {
		return nil, errors.New("audience and bundle ID are required")
	}
	signer, err := applejwt.New(config.SigningKey, config.KeyID, config.IssuerID, config.BundleID, "")
	if err != nil {
		return nil, err
	}
	return &JWSSignatureCreator{signer: signer, audience: audience, issuerID: config.IssuerID, bundleID: config.BundleID}, nil
}

func (s *JWSSignatureCreator) CreateSignature(featureClaims map[string]any) (string, error) {
	nonce, err := applejwt.Nonce()
	if err != nil {
		return "", err
	}
	claims := jwt.MapClaims{}
	for k, v := range featureClaims {
		claims[k] = v
	}
	claims["aud"], claims["iss"], claims["bid"], claims["iat"], claims["nonce"] = s.audience, s.issuerID, s.bundleID, time.Now().Unix(), nonce
	return s.signer.Sign(claims)
}

type PromotionalOfferV2SignatureCreator struct{ jws *JWSSignatureCreator }

func NewPromotionalOfferV2SignatureCreator(config SignatureConfig) (*PromotionalOfferV2SignatureCreator, error) {
	jws, err := NewJWSSignatureCreator("promotional-offer", config)
	if err != nil {
		return nil, err
	}
	return &PromotionalOfferV2SignatureCreator{jws}, nil
}
func (s *PromotionalOfferV2SignatureCreator) CreateSignature(productID, offerIdentifier string, transactionID *string) (string, error) {
	if productID == "" || offerIdentifier == "" {
		return "", errors.New("product ID and offer identifier are required")
	}
	claims := map[string]any{"productId": productID, "offerIdentifier": offerIdentifier}
	if transactionID != nil {
		claims["transactionId"] = *transactionID
	}
	return s.jws.CreateSignature(claims)
}

type IntroductoryOfferEligibilitySignatureCreator struct{ jws *JWSSignatureCreator }

func NewIntroductoryOfferEligibilitySignatureCreator(config SignatureConfig) (*IntroductoryOfferEligibilitySignatureCreator, error) {
	jws, err := NewJWSSignatureCreator("introductory-offer-eligibility", config)
	if err != nil {
		return nil, err
	}
	return &IntroductoryOfferEligibilitySignatureCreator{jws}, nil
}
func (s *IntroductoryOfferEligibilitySignatureCreator) CreateSignature(productID string, allowIntroductoryOffer bool, transactionID string) (string, error) {
	if productID == "" || transactionID == "" {
		return "", errors.New("product ID and transaction ID are required")
	}
	return s.jws.CreateSignature(map[string]any{"productId": productID, "allowIntroductoryOffer": allowIntroductoryOffer, "transactionId": transactionID})
}
