package app_store

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func int32Pointer(value int32) *int32 { return &value }

func TestSendConsumptionInformation(t *testing.T) {
	for _, sandbox := range []bool{false, true} {
		for _, percentage := range []*int32{nil, int32Pointer(0), int32Pointer(25000), int32Pointer(100000)} {
			key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			transport := testTransport(func(r *http.Request) (*http.Response, error) {
				wantBase := ProductionBaseURL
				if sandbox {
					wantBase = SandboxBaseURL
				}
				if r.URL.String() != wantBase+"/inApps/v2/transactions/consumption/123" || r.Method != http.MethodPut {
					t.Errorf("request: %s %s", r.Method, r.URL)
				}
				if r.Header.Get("Content-Type") != "application/json" || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
					t.Error("missing request headers")
				}
				var body map[string]any
				if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
					t.Fatal(e)
				}
				if body["customerConsented"] != true || body["sampleContentProvided"] != false || body["deliveryStatus"] != "DELIVERED" {
					t.Errorf("required fields: %#v", body)
				}
				value, has := body["consumptionPercentage"]
				if percentage == nil {
					if has {
						t.Error("nil percentage must be omitted")
					}
				} else if !has || value != float64(*percentage) {
					t.Errorf("percentage: %#v", value)
				}
				if _, has := body["refundPreference"]; has {
					t.Error("unspecified preference must be omitted")
				}
				return testResponse(http.StatusAccepted, ""), nil
			})
			options := []Options{WithHttpClient(&http.Client{Transport: transport})}
			if sandbox {
				options = append(options, WithSandbox())
			}
			c, e := NewClient(NewToken("kid", "issuer", "bundle", key), options...)
			if e != nil {
				t.Fatal(e)
			}
			if e = c.SendConsumptionInformation(context.Background(), "123", ConsumptionRequest{CustomerConsented: true, DeliveryStatus: DeliveryStatusDelivered, ConsumptionPercentage: percentage}); e != nil {
				t.Fatal(e)
			}
		}
	}
}

func TestConsumptionRequestValidation(t *testing.T) {
	good := ConsumptionRequest{CustomerConsented: true, DeliveryStatus: DeliveryStatusDelivered}
	cases := []ConsumptionRequest{
		{DeliveryStatus: DeliveryStatusDelivered},
		{CustomerConsented: true},
		{CustomerConsented: true, DeliveryStatus: "UNKNOWN"},
		{CustomerConsented: true, DeliveryStatus: DeliveryStatusDelivered, RefundPreference: "UNKNOWN"},
		{CustomerConsented: true, DeliveryStatus: DeliveryStatusDelivered, ConsumptionPercentage: int32Pointer(-1)},
		{CustomerConsented: true, DeliveryStatus: DeliveryStatusDelivered, ConsumptionPercentage: int32Pointer(100001)},
		{CustomerConsented: true, DeliveryStatus: DeliveryStatusUndeliveredOther, ConsumptionPercentage: int32Pointer(1)},
	}
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	c, _ := NewClient(NewToken("kid", "issuer", "bundle", key), WithHttpClient(&http.Client{Transport: testTransport(func(*http.Request) (*http.Response, error) { t.Fatal("invalid input sent to Apple"); return nil, nil })}))
	for _, request := range cases {
		if e := c.SendConsumptionInformation(context.Background(), "123", request); e == nil {
			t.Errorf("invalid request accepted: %#v", request)
		}
	}
	if e := c.SendConsumptionInformation(context.Background(), " ", good); e == nil {
		t.Error("empty transaction ID accepted")
	}
}

func TestConsumptionResponseErrors(t *testing.T) {
	request := ConsumptionRequest{CustomerConsented: true, DeliveryStatus: DeliveryStatusDelivered}
	for _, status := range []int{400, 401, 404, 429, 500, 200} {
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			if status == 400 {
				w.Write([]byte(`{"errorCode":4000001,"errorMessage":"invalid transaction"}`))
			} else {
				w.Write([]byte("rejected"))
			}
		})
		e := c.SendConsumptionInformation(context.Background(), "123", request)
		if e == nil || !strings.Contains(e.Error(), "HTTP "+strconv.Itoa(status)) {
			t.Errorf("HTTP %d: %v", status, e)
		}
		if status == 400 && (UnwrapError(e) == nil || UnwrapError(e).ErrorCode != 4000001) {
			t.Errorf("lost Apple error: %v", e)
		}
	}
	sentinel := errors.New("connection failed")
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	c, _ := NewClient(NewToken("kid", "issuer", "bundle", key), WithHttpClient(&http.Client{Transport: testTransport(func(*http.Request) (*http.Response, error) { return nil, sentinel })}))
	if e := c.SendConsumptionInformation(context.Background(), "123", request); !errors.Is(e, sentinel) {
		t.Fatalf("network error: %v", e)
	}
	c.token = NewToken("kid", "issuer", "bundle", nil)
	if e := c.SendConsumptionInformation(context.Background(), "123", request); e == nil {
		t.Fatal("signing error ignored")
	}
}

// Generate a trusted test certificate chain and sign both JWS layers, exercising
// the public notification parser instead of bypassing signature verification.
func notificationSigner(t *testing.T) func(jwt.MapClaims) string {
	t.Helper()
	now := time.Now()
	rootKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	intermediateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	leafKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	root := &x509.Certificate{SerialNumber: big.NewInt(900001), NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	intermediate := &x509.Certificate{SerialNumber: big.NewInt(900002), NotBefore: root.NotBefore, NotAfter: root.NotAfter, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	leaf := &x509.Certificate{SerialNumber: big.NewInt(900003), NotBefore: root.NotBefore, NotAfter: root.NotAfter, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	makeCert := func(template, parent *x509.Certificate, pub any, key *ecdsa.PrivateKey) ([]byte, *x509.Certificate) {
		der, e := x509.CreateCertificate(rand.Reader, template, parent, pub, key)
		if e != nil {
			t.Fatal(e)
		}
		cert, e := x509.ParseCertificate(der)
		if e != nil {
			t.Fatal(e)
		}
		return der, cert
	}
	rootDER, rootCert := makeCert(root, root, &rootKey.PublicKey, rootKey)
	intermediateDER, intermediateCert := makeCert(intermediate, rootCert, &intermediateKey.PublicKey, rootKey)
	leafDER, _ := makeCert(leaf, intermediateCert, &leafKey.PublicKey, intermediateKey)
	previous := X5CVerifier.roots
	roots := make(map[string]*x509.Certificate, len(previous)+1)
	for k, v := range previous {
		roots[k] = v
	}
	roots[rootCert.SerialNumber.String()] = rootCert
	X5CVerifier.roots = roots
	t.Cleanup(func() { X5CVerifier.roots = previous })
	return func(claims jwt.MapClaims) string {
		token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
		token.Header["x5c"] = []string{base64.StdEncoding.EncodeToString(leafDER), base64.StdEncoding.EncodeToString(intermediateDER), base64.StdEncoding.EncodeToString(rootDER)}
		signed, e := token.SignedString(leafKey)
		if e != nil {
			t.Fatal(e)
		}
		return signed
	}
}

func TestPurchaseAndConsumptionNotifications(t *testing.T) {
	sign := notificationSigner(t)
	for _, kind := range []NotificationType{NotificationTypeConsumptionRequest, NotificationTypeOneTimeCharge} {
		for _, productType := range []TransactionType{TransactionTypeConsumable, TransactionTypeNonConsumable, TransactionTypeNonRenewing} {
			transaction := sign(jwt.MapClaims{"transactionId": "123", "originalTransactionId": "original", "bundleId": "com.test.app", "environment": "Sandbox", "productId": "product", "type": productType, "appAccountToken": "user-token", "quantity": 1, "inAppOwnershipType": "PURCHASED"})
			notification := sign(jwt.MapClaims{"notificationType": kind, "notificationUUID": "notification-id", "version": "2.0", "data": map[string]any{"bundleId": "com.test.app", "environment": "Sandbox", "signedTransactionInfo": transaction, "consumptionRequestReason": "FULFILLMENT_ISSUE"}})
			body, _ := json.Marshal(SignedPayload{SignedPayload: notification})
			p, e := ParseNotificationRequest(body)
			if e != nil {
				t.Fatal(e)
			}
			var tx *JWSTransactionDecodedPayload
			if kind == NotificationTypeConsumptionRequest {
				tx, e = p.ConsumptionRequestTransaction()
			} else {
				tx, e = p.OneTimeChargeTransaction()
			}
			if e != nil {
				t.Fatal(e)
			}
			if tx.TransactionId != "123" || tx.AppAccountToken != "user-token" || tx.Type != productType || tx.InAppOwnerShipType != "PURCHASED" || p.Data.ConsumptionRequestReason != ConsumptionRequestReasonFulfillmentIssue {
				t.Fatalf("lost notification data: %#v %#v", p, tx)
			}
			p.Data.Environment = EnvironmentProduction
			if _, e = p.transactionFor(kind); e == nil {
				t.Fatal("environment mismatch accepted")
			}
			p.Data.Environment = EnvironmentSandbox
			p.Data.BundleId = "other.app"
			if _, e = p.transactionFor(kind); e == nil {
				t.Fatal("app mismatch accepted")
			}
			p.Data.BundleId = "com.test.app"
			p.Data.SignedTransactionInfo = "invalid.jwt.signature"
			if _, e = p.transactionFor(kind); e == nil {
				t.Fatal("invalid transaction accepted")
			}
			p.Data.SignedTransactionInfo = ""
			if _, e = p.transactionFor(kind); e == nil {
				t.Fatal("missing transaction accepted")
			}
			p.Data = nil
			if _, e = p.transactionFor(kind); e == nil {
				t.Fatal("missing data accepted")
			}
		}
	}
	var p *NotificationV2Payload
	if _, e := p.OneTimeChargeTransaction(); e == nil {
		t.Fatal("nil payload accepted")
	}
	p = &NotificationV2Payload{NotificationType: NotificationTypeTest}
	if _, e := p.ConsumptionRequestTransaction(); e == nil {
		t.Fatal("wrong event accepted")
	}
}
