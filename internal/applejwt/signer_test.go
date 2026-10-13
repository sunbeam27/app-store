package applejwt

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestDistinctAppleJWTClaimsAndConcurrentCache(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	for _, test := range []struct{ issuer, bundle, subject string }{{"issuer", "com.example", ""}, {"issuer", "", ""}, {"", "", "user"}} {
		signer, err := New(encoded, "key", test.issuer, test.bundle, test.subject)
		if err != nil {
			t.Fatal(err)
		}
		first, err := signer.Token(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := jwt.Parse(first, func(*jwt.Token) (any, error) { return &key.PublicKey, nil }, jwt.WithAudience("appstoreconnect-v1"))
		if err != nil {
			t.Fatal(err)
		}
		claims := parsed.Claims.(jwt.MapClaims)
		for claim, want := range map[string]string{"iss": test.issuer, "bid": test.bundle, "sub": test.subject} {
			got, exists := claims[claim]
			if want == "" && exists {
				t.Fatalf("unexpected %s claim: %v", claim, got)
			}
			if want != "" && got != want {
				t.Fatalf("%s: %v", claim, got)
			}
		}
		exp, _ := claims.GetExpirationTime()
		if time.Until(exp.Time) > 5*time.Minute {
			t.Fatal("token lifetime exceeds five minutes")
		}
		var group sync.WaitGroup
		for i := 0; i < 20; i++ {
			group.Add(1)
			go func() {
				defer group.Done()
				token, err := signer.Token(context.Background())
				if err != nil || token != first {
					t.Errorf("cache: %v", err)
				}
			}()
		}
		group.Wait()
	}
}
