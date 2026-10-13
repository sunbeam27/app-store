// Deprecated: new applications should import github.com/sunbeam27/app-store/signin.
package app_store

import (
	"github.com/sunbeam27/app-store/signin"
	"net/http"
)

const (
	AppleAuthURL   = signin.AppleAuthURL
	AppleTokenURL  = signin.AppleTokenURL
	AppleKeysURL   = signin.AppleKeysURL
	AppleRevokeURL = signin.AppleRevokeURL
	AppleIssuer    = signin.AppleIssuer
)

type AppleKey = signin.AppleKey
type AppleKeysResponse = signin.AppleKeysResponse
type AppleIDTokenClaims = signin.AppleIDTokenClaims
type AppleTokenResponse = signin.AppleTokenResponse
type AppleAuthClient = signin.AppleAuthClient

func NewAppleAuthClient(clientID, teamID, keyID, privateKey string) *AppleAuthClient {
	return signin.NewAppleAuthClient(clientID, teamID, keyID, privateKey)
}
func VerifyAppleIDToken(client *http.Client, clientID, idToken string) (*AppleIDTokenClaims, error) {
	return signin.VerifyAppleIDToken(client, clientID, idToken)
}
