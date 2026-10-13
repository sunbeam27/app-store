package appstoreserver

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sunbeam27/app-store/appstoreserver/model"
	"golang.org/x/crypto/ocsp"
)

type testChain struct {
	root, intermediate, leaf          *x509.Certificate
	rootKey, intermediateKey, leafKey *ecdsa.PrivateKey
}

func newSignedChain(t *testing.T, server string, validOID bool) testChain {
	t.Helper()
	now := time.Now()
	var chain testChain
	key := func() *ecdsa.PrivateKey {
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		return k
	}
	chain.rootKey, chain.intermediateKey, chain.leafKey = key(), key(), key()
	create := func(template, parent *x509.Certificate, public any, private *ecdsa.PrivateKey) *x509.Certificate {
		der, err := x509.CreateCertificate(rand.Reader, template, parent, public, private)
		if err != nil {
			t.Fatal(err)
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			t.Fatal(err)
		}
		return cert
	}
	root := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Test root"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	chain.root = create(root, root, &chain.rootKey.PublicKey, chain.rootKey)
	intermediate := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "Test WWDR"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtraExtensions: []pkix.Extension{{Id: asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 2, 1}, Value: []byte{5, 0}}}}
	if server != "" {
		intermediate.OCSPServer = []string{server + "/intermediate"}
	}
	chain.intermediate = create(intermediate, chain.root, &chain.intermediateKey.PublicKey, chain.rootKey)
	leaf := &x509.Certificate{SerialNumber: big.NewInt(3), Subject: pkix.Name{CommonName: "Test signer"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, BasicConstraintsValid: true}
	if validOID {
		leaf.ExtraExtensions = []pkix.Extension{{Id: asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 11, 1}, Value: []byte{5, 0}}}
	}
	if server != "" {
		leaf.OCSPServer = []string{server + "/leaf"}
	}
	chain.leaf = create(leaf, chain.intermediate, &chain.leafKey.PublicKey, chain.intermediateKey)
	return chain
}
func (c testChain) sign(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["x5c"] = []string{base64.StdEncoding.EncodeToString(c.leaf.Raw), base64.StdEncoding.EncodeToString(c.intermediate.Raw), base64.StdEncoding.EncodeToString(c.root.Raw)}
	signed, err := token.SignedString(c.leafKey)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func TestVerifierOnlineChecks(t *testing.T) {
	for _, mode := range []string{"good", "revoked", "network", "stale", "wrong-issuer", "unauthorized-responder"} {
		t.Run(mode, func(t *testing.T) {
			var chain testChain
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if mode == "network" {
					w.WriteHeader(503)
					return
				}
				cert, issuer, key := chain.leaf, chain.intermediate, chain.intermediateKey
				if r.URL.Path == "/intermediate" {
					cert, issuer, key = chain.intermediate, chain.root, chain.rootKey
				}
				now := time.Now()
				response := ocsp.Response{Status: ocsp.Good, SerialNumber: cert.SerialNumber, ThisUpdate: now.Add(-time.Minute), NextUpdate: now.Add(time.Hour)}
				if mode == "revoked" {
					response.Status = ocsp.Revoked
					response.RevokedAt = now.Add(-time.Minute)
				}
				if mode == "stale" {
					response.NextUpdate = now.Add(-time.Second)
				}
				responseIssuer, responder := issuer, issuer
				if mode == "wrong-issuer" {
					different := *issuer
					different.RawSubject = []byte("different issuer")
					responseIssuer = &different
				}
				if mode == "unauthorized-responder" {
					responderKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
					if err != nil {
						t.Error(err)
						w.WriteHeader(500)
						return
					}
					template := &x509.Certificate{SerialNumber: big.NewInt(4), Subject: pkix.Name{CommonName: "Unrelated signer"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
					der, err := x509.CreateCertificate(rand.Reader, template, issuer, &responderKey.PublicKey, key)
					if err != nil {
						t.Error(err)
						w.WriteHeader(500)
						return
					}
					responder, err = x509.ParseCertificate(der)
					if err != nil {
						t.Error(err)
						w.WriteHeader(500)
						return
					}
					response.Certificate = responder
					key = responderKey
				}
				data, err := ocsp.CreateResponse(responseIssuer, responder, response, key)
				if err != nil {
					t.Error(err)
					w.WriteHeader(500)
					return
				}
				w.Write(data)
			}))
			defer server.Close()
			chain = newSignedChain(t, server.URL, true)
			v, err := NewSignedDataVerifier(VerifierConfig{RootCertificates: [][]byte{chain.root.Raw}, BundleID: "com.example", Environment: model.EnvironmentSandbox, OnlineChecks: true})
			if err != nil {
				t.Fatal(err)
			}
			signed := chain.sign(t, jwt.MapClaims{"bundleId": "com.example", "environment": "Sandbox", "signedDate": time.Now().UnixMilli()})
			_, err = v.VerifyAndDecodeTransaction(context.Background(), signed)
			if mode == "good" {
				if err != nil {
					t.Fatal(err)
				}
				if _, err := v.VerifyAndDecodeTransaction(context.Background(), signed); err != nil {
					t.Fatal(err)
				}
				if requests.Load() != 2 {
					t.Fatalf("verified chain was not cached: %d OCSP requests", requests.Load())
				}
			} else if mode == "revoked" || mode == "wrong-issuer" || mode == "unauthorized-responder" {
				requireStatus(t, err, VerificationInvalidChain)
			} else {
				requireStatus(t, err, VerificationRetryableFailure)
			}
		})
	}
}

func TestVerifierRequiresAppleSigningOIDAndTrustedRoot(t *testing.T) {
	for _, validOID := range []bool{true, false} {
		chain := newSignedChain(t, "", validOID)
		v, err := NewSignedDataVerifier(VerifierConfig{RootCertificates: [][]byte{chain.root.Raw}, BundleID: "com.example", Environment: model.EnvironmentSandbox})
		if err != nil {
			t.Fatal(err)
		}
		signed := chain.sign(t, jwt.MapClaims{"bundleId": "com.example", "environment": "Sandbox", "signedDate": time.Now().UnixMilli()})
		_, err = v.VerifyAndDecodeTransaction(context.Background(), signed)
		if validOID {
			if err != nil {
				t.Fatal(err)
			}
		} else {
			requireStatus(t, err, VerificationInvalidChain)
		}
		other := newSignedChain(t, "", true)
		v, err = NewSignedDataVerifier(VerifierConfig{RootCertificates: [][]byte{other.root.Raw}, BundleID: "com.example", Environment: model.EnvironmentSandbox})
		if err != nil {
			t.Fatal(err)
		}
		_, err = v.VerifyAndDecodeTransaction(context.Background(), signed)
		requireStatus(t, err, VerificationInvalidChain)
	}
}
