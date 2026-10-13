package appstoreserver

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sunbeam27/app-store/appstoreserver/model"
	"github.com/sunbeam27/app-store/internal/jsonmodel"
	"golang.org/x/crypto/ocsp"
)

type VerificationStatus string

const (
	VerificationFailure              VerificationStatus = "VERIFICATION_FAILURE"
	VerificationInvalidCertificate   VerificationStatus = "INVALID_CERTIFICATE"
	VerificationInvalidChain         VerificationStatus = "INVALID_CHAIN"
	VerificationInvalidChainLength   VerificationStatus = "INVALID_CHAIN_LENGTH"
	VerificationInvalidAppIdentifier VerificationStatus = "INVALID_APP_IDENTIFIER"
	VerificationInvalidEnvironment   VerificationStatus = "INVALID_ENVIRONMENT"
	VerificationRetryableFailure     VerificationStatus = "RETRYABLE_VERIFICATION_FAILURE"
)

type VerificationError struct {
	Status VerificationStatus
	Err    error
}

func (e *VerificationError) Error() string {
	return fmt.Sprintf("Apple signed data: %s: %v", e.Status, e.Err)
}
func (e *VerificationError) Unwrap() error { return e.Err }
func verificationError(status VerificationStatus, message string) error {
	return &VerificationError{Status: status, Err: errors.New(message)}
}

type VerifierConfig struct {
	// RootCertificates contains trusted Apple roots in DER or PEM format.
	RootCertificates [][]byte
	BundleID         string
	AppAppleID       *int64
	Environment      model.Environment
	// OnlineChecks enables OCSP and verifies certificates at the current time.
	// Otherwise verification uses the payload's signing date, matching Apple.
	OnlineChecks bool
	HTTPClient   *http.Client
}

type SignedDataVerifier struct {
	roots          *x509.CertPool
	bundleID       string
	appAppleID     *int64
	environment    model.Environment
	online         bool
	client         *http.Client
	cacheMu        sync.Mutex
	verifiedChains map[[32]byte]time.Time
}

func NewSignedDataVerifier(config VerifierConfig) (*SignedDataVerifier, error) {
	if config.BundleID == "" {
		return nil, errors.New("bundle ID is required")
	}
	switch config.Environment {
	case model.EnvironmentProduction, model.EnvironmentSandbox, model.EnvironmentXcode, model.EnvironmentLocalTesting:
	default:
		return nil, errors.New("invalid verifier environment")
	}
	if config.Environment == model.EnvironmentProduction && config.AppAppleID == nil {
		return nil, errors.New("app Apple ID is required in Production")
	}
	roots := x509.NewCertPool()
	for _, data := range config.RootCertificates {
		if block, _ := pem.Decode(data); block != nil {
			data = block.Bytes
		}
		cert, err := x509.ParseCertificate(data)
		if err != nil {
			return nil, fmt.Errorf("parse root certificate: %w", err)
		}
		roots.AddCert(cert)
	}
	if len(config.RootCertificates) == 0 {
		return nil, errors.New("at least one trusted root certificate is required")
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	var appAppleID *int64
	if config.AppAppleID != nil {
		value := *config.AppAppleID
		appAppleID = &value
	}
	return &SignedDataVerifier{roots: roots, bundleID: config.BundleID, appAppleID: appAppleID, environment: config.Environment, online: config.OnlineChecks, client: &copyClient, verifiedChains: make(map[[32]byte]time.Time)}, nil
}

func value[T any](v *T) T {
	if v != nil {
		return *v
	}
	var zero T
	return zero
}

func (v *SignedDataVerifier) checkApp(bundleID string, appAppleID *int64, environment model.Environment, checkID bool) error {
	if bundleID != v.bundleID {
		return verificationError(VerificationInvalidAppIdentifier, "bundle ID mismatch")
	}
	if checkID && v.environment == model.EnvironmentProduction && (appAppleID == nil || *appAppleID != *v.appAppleID) {
		return verificationError(VerificationInvalidAppIdentifier, "app Apple ID mismatch")
	}
	return v.checkEnvironment(environment)
}
func (v *SignedDataVerifier) checkEnvironment(environment model.Environment) error {
	if environment != v.environment {
		return verificationError(VerificationInvalidEnvironment, "environment mismatch")
	}
	return nil
}

func (v *SignedDataVerifier) VerifyAndDecodeTransaction(ctx context.Context, signed string) (*model.JWSTransactionDecodedPayload, error) {
	var result model.JWSTransactionDecodedPayload
	if err := v.decode(ctx, signed, &result, false); err != nil {
		return nil, err
	}
	if err := v.checkApp(value(result.BundleID), nil, value(result.Environment), false); err != nil {
		return nil, err
	}
	return &result, nil
}

func (v *SignedDataVerifier) VerifyAndDecodeRenewalInfo(ctx context.Context, signed string) (*model.JWSRenewalInfoDecodedPayload, error) {
	var result model.JWSRenewalInfoDecodedPayload
	if err := v.decode(ctx, signed, &result, false); err != nil {
		return nil, err
	}
	if err := v.checkEnvironment(value(result.Environment)); err != nil {
		return nil, err
	}
	return &result, nil
}

func (v *SignedDataVerifier) VerifyAndDecodeNotification(ctx context.Context, signed string) (*model.ResponseBodyV2DecodedPayload, error) {
	var result model.ResponseBodyV2DecodedPayload
	if err := v.decode(ctx, signed, &result, false); err != nil {
		return nil, err
	}
	var bundle string
	var appID *int64
	var environment model.Environment
	switch {
	case result.Data != nil:
		bundle, appID, environment = value(result.Data.BundleID), result.Data.AppAppleID, value(result.Data.Environment)
	case result.Summary != nil:
		bundle, appID, environment = value(result.Summary.BundleID), result.Summary.AppAppleID, value(result.Summary.Environment)
	case result.ExternalPurchaseToken != nil:
		bundle, appID = value(result.ExternalPurchaseToken.BundleID), result.ExternalPurchaseToken.AppAppleID
		environment = model.EnvironmentProduction
		if strings.HasPrefix(value(result.ExternalPurchaseToken.ExternalPurchaseID), "SANDBOX") {
			environment = model.EnvironmentSandbox
		}
	case result.AppData != nil:
		bundle, appID, environment = value(result.AppData.BundleID), result.AppData.AppAppleID, value(result.AppData.Environment)
	}
	if err := v.checkApp(bundle, appID, environment, true); err != nil {
		return nil, err
	}
	return &result, nil
}

func (v *SignedDataVerifier) VerifyAndDecodeAppTransaction(ctx context.Context, signed string) (*model.AppTransaction, error) {
	var result model.AppTransaction
	if err := v.decode(ctx, signed, &result, true); err != nil {
		return nil, err
	}
	if err := v.checkApp(value(result.BundleID), result.AppAppleID, value(result.ReceiptType), true); err != nil {
		return nil, err
	}
	return &result, nil
}

func (v *SignedDataVerifier) VerifyAndDecodeRealtimeRequest(ctx context.Context, signed string) (*model.DecodedRealtimeRequestBody, error) {
	var result model.DecodedRealtimeRequestBody
	if err := v.decode(ctx, signed, &result, false); err != nil {
		return nil, err
	}
	if v.environment == model.EnvironmentProduction && result.AppAppleID != *v.appAppleID {
		return nil, verificationError(VerificationInvalidAppIdentifier, "app Apple ID mismatch")
	}
	if err := v.checkEnvironment(result.Environment); err != nil {
		return nil, err
	}
	return &result, nil
}

func (v *SignedDataVerifier) decode(ctx context.Context, signed string, result any, appTransaction bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	parts := strings.Split(signed, ".")
	if len(parts) != 3 {
		return verificationError(VerificationFailure, "invalid JWS format")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return &VerificationError{VerificationFailure, err}
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return &VerificationError{VerificationFailure, err}
	}
	var header struct {
		Alg string   `json:"alg"`
		X5C []string `json:"x5c"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return &VerificationError{VerificationFailure, err}
	}
	if err := json.Unmarshal(payload, result); err != nil {
		return &VerificationError{VerificationFailure, err}
	}
	// These explicit testing environments bypass signatures; the public methods
	// must still validate the decoded environment and application identifiers.
	if v.environment == model.EnvironmentXcode || v.environment == model.EnvironmentLocalTesting {
		return nil
	}
	if header.Alg != "ES256" {
		return verificationError(VerificationFailure, "ES256 is required")
	}
	if header.X5C == nil {
		return verificationError(VerificationFailure, "x5c header is missing")
	}
	if len(header.X5C) != 3 {
		return verificationError(VerificationInvalidChainLength, "expected three x5c certificates")
	}
	certs := make([]*x509.Certificate, 3)
	for i, encoded := range header.X5C {
		der, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return &VerificationError{VerificationInvalidCertificate, err}
		}
		certs[i], err = x509.ParseCertificate(der)
		if err != nil {
			return &VerificationError{VerificationInvalidCertificate, err}
		}
	}
	if !hasOID(certs[0], asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 11, 1}) || !hasOID(certs[1], asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 2, 1}) {
		return verificationError(VerificationInvalidChain, "missing Apple signing certificate extensions")
	}
	now := time.Now()
	effective := now
	if !v.online {
		var dates struct {
			SignedDate          *int64 `json:"signedDate"`
			ReceiptCreationDate *int64 `json:"receiptCreationDate"`
		}
		normalized, err := jsonmodel.NormalizeTimestamps(payload, []string{"signedDate", "receiptCreationDate"})
		if err != nil {
			return &VerificationError{VerificationFailure, err}
		}
		if err := json.Unmarshal(normalized, &dates); err != nil {
			return &VerificationError{VerificationFailure, err}
		}
		date := dates.SignedDate
		if appTransaction {
			date = dates.ReceiptCreationDate
		}
		if date != nil {
			effective = time.UnixMilli(*date)
		}
	}
	intermediates := x509.NewCertPool()
	intermediates.AddCert(certs[1])
	chains, err := certs[0].Verify(x509.VerifyOptions{Roots: v.roots, Intermediates: intermediates, CurrentTime: effective, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}})
	if err != nil {
		return &VerificationError{VerificationInvalidChain, err}
	}
	key, ok := certs[0].PublicKey.(*ecdsa.PublicKey)
	if !ok || key.Curve != elliptic.P256() {
		return verificationError(VerificationFailure, "signing certificate must use P-256")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(signature) != 64 {
		return verificationError(VerificationFailure, "invalid ES256 signature")
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if !ecdsa.Verify(key, digest[:], new(big.Int).SetBytes(signature[:32]), new(big.Int).SetBytes(signature[32:])) {
		return verificationError(VerificationFailure, "signature mismatch")
	}
	var registered jwt.MapClaims
	if err := json.Unmarshal(payload, &registered); err != nil {
		return &VerificationError{VerificationFailure, err}
	}
	if err := jwt.NewValidator(jwt.WithIssuedAt()).Validate(registered); err != nil {
		return &VerificationError{VerificationFailure, err}
	}
	if v.online {
		cacheKey := sha256.Sum256([]byte(strings.Join(header.X5C, ".")))
		v.cacheMu.Lock()
		expires, cached := v.verifiedChains[cacheKey]
		v.cacheMu.Unlock()
		if cached && now.Before(expires) {
			return nil
		}
		chain := chains[0]
		for i := 0; i < len(chain)-1; i++ {
			if err := v.checkOCSP(ctx, chain[i], chain[i+1], now); err != nil {
				return err
			}
		}
		expires = now.Add(15 * time.Minute)
		for _, cert := range chain {
			if cert.NotAfter.Before(expires) {
				expires = cert.NotAfter
			}
		}
		v.cacheMu.Lock()
		for key, expiration := range v.verifiedChains {
			if !now.Before(expiration) {
				delete(v.verifiedChains, key)
			}
		}
		if len(v.verifiedChains) >= 32 {
			var oldest [32]byte
			var earliest time.Time
			for key, expiration := range v.verifiedChains {
				if earliest.IsZero() || expiration.Before(earliest) {
					oldest, earliest = key, expiration
				}
			}
			delete(v.verifiedChains, oldest)
		}
		v.verifiedChains[cacheKey] = expires
		v.cacheMu.Unlock()
	}
	return nil
}

func hasOID(cert *x509.Certificate, oid asn1.ObjectIdentifier) bool {
	for _, extension := range cert.Extensions {
		if extension.Id.Equal(oid) && !extension.Critical {
			return true
		}
	}
	return false
}

func (v *SignedDataVerifier) checkOCSP(ctx context.Context, cert, issuer *x509.Certificate, now time.Time) error {
	request, err := ocsp.CreateRequest(cert, issuer, nil)
	if err != nil {
		return &VerificationError{VerificationInvalidCertificate, err}
	}
	var last error
	for _, server := range cert.OCSPServer {
		req, err := http.NewRequestWithContext(ctx, "POST", server, bytes.NewReader(request))
		if err != nil {
			last = err
			continue
		}
		if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
			last = errors.New("invalid OCSP URL")
			continue
		}
		req.Header.Set("Content-Type", "application/ocsp-request")
		req.Header.Set("Accept", "application/ocsp-response")
		resp, err := v.client.Do(req)
		if err != nil {
			last = err
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
		resp.Body.Close()
		if readErr != nil {
			last = readErr
			continue
		}
		if resp.StatusCode != 200 || len(data) > 1<<20 {
			last = fmt.Errorf("OCSP responder returned HTTP %d", resp.StatusCode)
			continue
		}
		status, err := ocsp.ParseResponseForCert(data, cert, issuer)
		if err != nil {
			return &VerificationError{VerificationInvalidChain, err}
		}
		if err := verifyOCSPIssuer(status, cert, issuer); err != nil {
			return &VerificationError{VerificationInvalidChain, err}
		}
		if responder := status.Certificate; responder != nil && !responder.Equal(issuer) {
			permitted := false
			for _, usage := range responder.ExtKeyUsage {
				if usage == x509.ExtKeyUsageOCSPSigning {
					permitted = true
				}
			}
			if !permitted {
				return verificationError(VerificationInvalidChain, "delegated OCSP responder lacks OCSP signing authorization")
			}
			roots := x509.NewCertPool()
			roots.AddCert(issuer)
			if _, err := responder.Verify(x509.VerifyOptions{Roots: roots, CurrentTime: now, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageOCSPSigning}}); err != nil {
				return &VerificationError{VerificationInvalidChain, err}
			}
		}
		if status.Status == ocsp.Revoked {
			return verificationError(VerificationInvalidChain, "certificate is revoked")
		}
		if status.Status != ocsp.Good {
			last = errors.New("OCSP status is unknown")
			continue
		}
		if status.ThisUpdate.After(now.Add(time.Minute)) || (!status.NextUpdate.IsZero() && !status.NextUpdate.After(now)) || (status.NextUpdate.IsZero() && status.ThisUpdate.Before(now.Add(-24*time.Hour))) {
			last = errors.New("OCSP response is not current")
			continue
		}
		return nil
	}
	if last == nil {
		last = errors.New("certificate has no OCSP responder")
	}
	return &VerificationError{VerificationRetryableFailure, last}
}

// x/crypto verifies the OCSP signature and serial number. Bind CertID's issuer
// hashes as well, so a same-serial response for another issuer cannot be reused.
func verifyOCSPIssuer(response *ocsp.Response, cert, issuer *x509.Certificate) error {
	var data struct {
		Version     int `asn1:"optional,default:0,explicit,tag:0"`
		ResponderID asn1.RawValue
		ProducedAt  time.Time `asn1:"generalized"`
		Responses   []asn1.RawValue
		Extensions  []pkix.Extension `asn1:"explicit,tag:1,optional"`
	}
	if _, err := asn1.Unmarshal(response.TBSResponseData, &data); err != nil {
		return err
	}
	var publicKey struct {
		Algorithm pkix.AlgorithmIdentifier
		Key       asn1.BitString
	}
	if _, err := asn1.Unmarshal(issuer.RawSubjectPublicKeyInfo, &publicKey); err != nil {
		return err
	}
	if !response.IssuerHash.Available() {
		return errors.New("unsupported OCSP issuer hash")
	}
	hash := response.IssuerHash.New()
	hash.Write(issuer.RawSubject)
	nameHash := hash.Sum(nil)
	hash.Reset()
	hash.Write(publicKey.Key.RightAlign())
	keyHash := hash.Sum(nil)
	for _, raw := range data.Responses {
		var id struct {
			Algorithm         pkix.AlgorithmIdentifier
			NameHash, KeyHash []byte
			Serial            *big.Int
		}
		if _, err := asn1.Unmarshal(raw.Bytes, &id); err != nil {
			return err
		}
		if id.Serial != nil && id.Serial.Cmp(cert.SerialNumber) == 0 {
			if !bytes.Equal(id.NameHash, nameHash) || !bytes.Equal(id.KeyHash, keyHash) {
				return errors.New("OCSP issuer does not match certificate issuer")
			}
			return nil
		}
	}
	return errors.New("OCSP response does not identify the certificate")
}
