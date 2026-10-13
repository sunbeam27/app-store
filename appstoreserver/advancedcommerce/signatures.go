// Package advancedcommerce signs StoreKit Advanced Commerce in-app requests.
package advancedcommerce

import (
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/sunbeam27/app-store/appstoreserver/offers"
)

type AdvancedCommerceInAppSignatureCreator struct{ jws *offers.JWSSignatureCreator }

func NewAdvancedCommerceInAppSignatureCreator(config offers.SignatureConfig) (*AdvancedCommerceInAppSignatureCreator, error) {
	jws, err := offers.NewJWSSignatureCreator("advanced-commerce-api", config)
	if err != nil {
		return nil, err
	}
	return &AdvancedCommerceInAppSignatureCreator{jws}, nil
}

// CreateSignature encodes request as Base64 JSON in the request claim, matching
// Apple's library. Pass a model.AdvancedCommerce*InAppRequest/CreateRequest.
func (s *AdvancedCommerceInAppSignatureCreator) CreateSignature(request any) (string, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	if string(data) == "null" {
		return "", errors.New("advanced commerce request is required")
	}
	var payload struct {
		Operation string `json:"operation"`
		Version   string `json:"version"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", err
	}
	if payload.Operation == "" || payload.Version == "" {
		return "", errors.New("advanced commerce operation and version are required")
	}
	return s.jws.CreateSignature(map[string]any{"request": base64.StdEncoding.EncodeToString(data)})
}
