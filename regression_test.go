package app_store

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"
)

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func testResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestMalformedNotificationHeaders(t *testing.T) {
	for _, header := range []string{"null", "{}", `{"alg":"ES256","x5c":[]}`, `{"alg":"ES256","x5c":["one"]}`, `{"alg":"ES256","x5c":["one","two"]}`} {
		t.Run(header, func(t *testing.T) {
			token := base64.RawURLEncoding.EncodeToString([]byte(header)) + ".e30.signature"
			if _, err := DecodeNotificationV2(token); err == nil {
				t.Fatal("expected malformed header error")
			}
		})
	}
}

func TestFreshTokenCacheAndRefresh(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	token := NewToken("kid", "issuer", "bundle", key)
	first, err := token.bearer()
	if err != nil || first == "Bearer " {
		t.Fatalf("first request: %q %v", first, err)
	}
	second, err := token.bearer()
	if err != nil || first != second {
		t.Fatalf("cached request: %q %v", second, err)
	}
	token.exp = time.Now().Add(-time.Second)
	token.token = "expired"
	refreshed, err := token.signedString()
	if err != nil || refreshed == "expired" || refreshed == "" || token.token != refreshed {
		t.Fatalf("refresh: %q %v", refreshed, err)
	}
}

func TestAPINetworkAndSigningErrors(t *testing.T) {
	sentinel := errors.New("network unavailable")
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	c, _ := NewClient(NewToken("kid", "issuer", "bundle", key), WithHttpClient(&http.Client{Transport: testTransport(func(*http.Request) (*http.Response, error) { return nil, sentinel })}))
	calls := []func() error{
		func() error { _, e := c.LookupOrder(context.Background(), "id"); return e },
		func() error { _, e := c.GetTransactionInfo(context.Background(), "id"); return e },
		func() error { _, e := c.GetTransactionHistory(context.Background(), "id", ""); return e },
		func() error { _, e := c.GetRefundHistory(context.Background(), "id", ""); return e },
		func() error { _, e := c.GetAllSubscriptionStatuses(context.Background(), "id"); return e },
	}
	for i, call := range calls {
		if e := call(); !errors.Is(e, sentinel) {
			t.Fatalf("call %d: %v", i, e)
		}
	}
	c.token = NewToken("kid", "issuer", "bundle", nil)
	for i, call := range calls {
		if e := call(); e == nil {
			t.Fatalf("call %d accepted invalid signing key", i)
		}
	}
}

func TestHistoryRevisionEscaping(t *testing.T) {
	revision := "a+b&next=x? /"
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("revision") != revision || len(r.URL.Query()) != 1 {
			t.Errorf("query corrupted: %s", r.URL.RawQuery)
		}
		if r.URL.Path != "/inApps/v2/history/id" && r.URL.Path != "/inApps/v2/refund/lookup/id" {
			t.Errorf("wrong path: %s", r.URL.Path)
		}
		w.Write([]byte("{}"))
	})
	if _, e := c.GetTransactionHistory(context.Background(), "id", revision); e != nil {
		t.Fatal(e)
	}
	if _, e := c.GetRefundHistory(context.Background(), "id", revision); e != nil {
		t.Fatal(e)
	}
}

func TestAppleTokenOperations(t *testing.T) {
	a := NewAppleAuthClient("client", "team", "kid", generateTestP8Key(t))
	for _, operation := range []string{"exchange", "refresh", "revoke"} {
		t.Run(operation, func(t *testing.T) {
			for _, status := range []int{200, 400} {
				a.WithHTTPClient(&http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
					if r.Method != "POST" {
						t.Errorf("method %s", r.Method)
					}
					if err := r.ParseForm(); err != nil {
						t.Fatal(err)
					}
					if r.Form.Get("client_id") != "client" || r.Form.Get("client_secret") == "" {
						t.Error("missing client credentials")
					}
					wantURL := AppleTokenURL
					switch operation {
					case "exchange":
						if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code") != "code" || r.Form.Get("redirect_uri") != "https://example.com/callback" {
							t.Error("exchange form")
						}
					case "refresh":
						if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "refresh" {
							t.Error("refresh form")
						}
					case "revoke":
						wantURL = AppleRevokeURL
						if r.Form.Get("token") != "token" || r.Form.Get("token_type_hint") != "access_token" {
							t.Error("revoke form")
						}
					}
					if r.URL.String() != wantURL {
						t.Errorf("URL %s", r.URL)
					}
					return testResponse(status, `{"access_token":"access","refresh_token":"refresh","id_token":"id"}`), nil
				})})
				var err error
				switch operation {
				case "exchange":
					var resp *AppleTokenResponse
					resp, err = a.ExchangeCode("code", "https://example.com/callback")
					if err == nil && resp.AccessToken != "access" {
						t.Error("response not decoded")
					}
				case "refresh":
					var resp *AppleTokenResponse
					resp, err = a.RefreshAccessToken("refresh")
					if err == nil && resp.AccessToken != "access" {
						t.Error("response not decoded")
					}
				case "revoke":
					err = a.RevokeToken("token", "access_token")
				}
				if (err != nil) != (status != 200) {
					t.Fatalf("status %d: %v", status, err)
				}
			}
		})
	}
}

func TestAppleVerificationPublicMethods(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks, _ := json.Marshal(AppleKeysResponse{Keys: []AppleKey{{Kid: "key", Kty: "RSA", Use: "sig", Alg: "RS256", N: base64.RawURLEncoding.EncodeToString(key.N.Bytes()), E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}})
	a := NewAppleAuthClient("client", "team", "kid", "")
	a.WithHTTPClient(&http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != AppleKeysURL {
			t.Errorf("JWKS URL: %s", r.URL)
		}
		return testResponse(200, string(jwks)), nil
	})})
	for _, tc := range []string{"valid", "issuer", "audience", "expired", "missing-exp", "subject", "nonce", "algorithm", "signature", "kid"} {
		t.Run(tc, func(t *testing.T) {
			claims := jwt.MapClaims{"iss": AppleIssuer, "aud": "client", "exp": time.Now().Add(time.Hour).Unix(), "sub": "user", "nonce": "expected"}
			method := jwt.SigningMethod(jwt.SigningMethodRS256)
			var signingKey interface{} = key
			switch tc {
			case "issuer":
				claims["iss"] = "other"
			case "audience":
				claims["aud"] = "other"
			case "expired":
				claims["exp"] = time.Now().Add(-time.Hour).Unix()
			case "missing-exp":
				delete(claims, "exp")
			case "subject":
				delete(claims, "sub")
			case "nonce":
				claims["nonce"] = "other"
			case "algorithm":
				method = jwt.SigningMethodRS512
			case "signature":
				signingKey, _ = rsa.GenerateKey(rand.Reader, 2048)
			}
			token := jwt.NewWithClaims(method, claims)
			token.Header["kid"] = "key"
			if tc == "kid" {
				token.Header["kid"] = "unknown"
			}
			signed, e := token.SignedString(signingKey)
			if e != nil {
				t.Fatal(e)
			}
			result, e := a.VerifyIDTokenWithNonce(signed, "expected")
			if tc == "valid" {
				if e != nil || result.Subject != "user" {
					t.Fatalf("valid: %v", e)
				}
			} else if e == nil {
				t.Fatal("invalid token accepted")
			}
		})
	}
	if _, e := a.VerifyIDTokenWithNonce("", ""); e == nil {
		t.Fatal("empty expected nonce accepted")
	}
	a.WithHTTPClient(&http.Client{Transport: testTransport(func(*http.Request) (*http.Response, error) { return testResponse(503, string(jwks)), nil })})
	if _, e := a.VerifyIDToken("invalid"); e == nil {
		t.Fatal("JWKS HTTP failure ignored")
	}
}
