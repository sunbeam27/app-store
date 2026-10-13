package offers

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func signatureKey(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return key, string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func TestLegacyOfferSignatureUsesApplePayloadAndDER(t *testing.T) {
	key, encoded := signatureKey(t)
	creator, err := NewPromotionalOfferSignatureCreator(encoded, "key", "com.example")
	if err != nil {
		t.Fatal(err)
	}
	nonce := "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE"
	signature, err := creator.CreateSignature("product", "offer", "ACCOUNT", nonce, 1700000000000)
	if err != nil {
		t.Fatal(err)
	}
	payload := strings.Join([]string{"com.example", "key", "product", "offer", "account", strings.ToLower(nonce), "1700000000000"}, "\u2063")
	digest := sha256.Sum256([]byte(payload))
	der, err := base64.StdEncoding.DecodeString(signature)
	if err != nil || !ecdsa.VerifyASN1(&key.PublicKey, digest[:], der) {
		t.Fatal("signature differs from Apple's legacy offer format")
	}
}

func TestOfferJWSClaimsAndFreshNonces(t *testing.T) {
	key, encoded := signatureKey(t)
	config := SignatureConfig{SigningKey: encoded, KeyID: "key", IssuerID: "issuer", BundleID: "com.example"}
	v2, err := NewPromotionalOfferV2SignatureCreator(config)
	if err != nil {
		t.Fatal(err)
	}
	intro, err := NewIntroductoryOfferEligibilitySignatureCreator(config)
	if err != nil {
		t.Fatal(err)
	}
	var previousNonce string
	for _, test := range []struct {
		audience string
		sign     func() (string, error)
	}{
		{"promotional-offer", func() (string, error) { return v2.CreateSignature("product", "offer", nil) }},
		{"promotional-offer", func() (string, error) { return v2.CreateSignature("product", "offer", nil) }},
		{"introductory-offer-eligibility", func() (string, error) { return intro.CreateSignature("product", false, "transaction") }},
	} {
		signed, err := test.sign()
		if err != nil {
			t.Fatal(err)
		}
		token, err := jwt.Parse(signed, func(*jwt.Token) (any, error) { return &key.PublicKey, nil }, jwt.WithAudience(test.audience), jwt.WithIssuer("issuer"))
		if err != nil {
			t.Fatal(err)
		}
		claims := token.Claims.(jwt.MapClaims)
		nonce, ok := claims["nonce"].(string)
		if !ok || !validUUID(nonce) || nonce == previousNonce {
			t.Fatal("nonce must be a fresh UUID")
		}
		previousNonce = nonce
		if claims["bid"] != "com.example" || claims["iat"] == nil {
			t.Fatal("missing StoreKit claims")
		}
		if test.audience == "introductory-offer-eligibility" && claims["allowIntroductoryOffer"] != false {
			t.Fatal("false eligibility was lost")
		}
	}
}
