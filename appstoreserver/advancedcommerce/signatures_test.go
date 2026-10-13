package advancedcommerce

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sunbeam27/app-store/appstoreserver/model"
	"github.com/sunbeam27/app-store/appstoreserver/offers"
)

func TestAdvancedCommerceEncodedRequestAndInheritedFields(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	creator, err := NewAdvancedCommerceInAppSignatureCreator(offers.SignatureConfig{SigningKey: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), KeyID: "key", IssuerID: "issuer", BundleID: "com.example"})
	if err != nil {
		t.Fatal(err)
	}
	request := model.AdvancedCommerceSubscriptionCreateRequest{Currency: "USD", TaxCode: "tax", Period: model.AdvancedCommercePeriodP1M, RequestInfo: model.AdvancedCommerceRequestInfo{RequestReferenceID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"}}
	signed, err := creator.CreateSignature(request)
	if err != nil {
		t.Fatal(err)
	}
	token, err := jwt.Parse(signed, func(*jwt.Token) (any, error) { return &key.PublicKey, nil }, jwt.WithAudience("advanced-commerce-api"))
	if err != nil {
		t.Fatal(err)
	}
	claims := token.Claims.(jwt.MapClaims)
	payload, err := base64.StdEncoding.DecodeString(claims["request"].(string))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["operation"] != "CREATE_SUBSCRIPTION" || decoded["version"] != "1" || decoded["currency"] != "USD" || decoded["requestInfo"].(map[string]any)["requestReferenceId"] != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Fatalf("request encoding: %s", payload)
	}
	if _, err := creator.CreateSignature(nil); err == nil {
		t.Fatal("nil request accepted")
	}
}
