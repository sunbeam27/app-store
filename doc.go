// Package app_store retains the original SDK API for source compatibility.
//
// Deprecated: use the appstoreserver, appstoreconnect, and signin packages for
// new integrations. Legacy DecodeClaims does not bind payloads to a configured
// application; appstoreserver.SignedDataVerifier performs that validation.
package app_store
