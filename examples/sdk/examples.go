// Package sdkexamples contains compiling examples of the new SDK packages.
package sdkexamples

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sunbeam27/app-store/appstoreconnect"
	connectmodel "github.com/sunbeam27/app-store/appstoreconnect/model"
	"github.com/sunbeam27/app-store/appstoreserver"
	servermodel "github.com/sunbeam27/app-store/appstoreserver/model"
)

func VerifiedTransaction(ctx context.Context, config appstoreserver.ClientConfig, verification appstoreserver.VerifierConfig, transactionID string) (*servermodel.JWSTransactionDecodedPayload, error) {
	client, err := appstoreserver.NewAppStoreServerAPIClient(config)
	if err != nil {
		return nil, err
	}
	verifier, err := appstoreserver.NewSignedDataVerifier(verification)
	if err != nil {
		return nil, err
	}
	response, err := client.GetTransactionInfo(ctx, transactionID)
	if err != nil {
		return nil, err
	}
	if response.SignedTransactionInfo == nil {
		return nil, fmt.Errorf("transaction response has no signed data")
	}
	return verifier.VerifyAndDecodeTransaction(ctx, *response.SignedTransactionInfo)
}

func CreateConsumable(ctx context.Context, client *appstoreconnect.Client, appID, name, productID string) (*connectmodel.InAppPurchaseV2Response, error) {
	return client.CreateInAppPurchase(ctx, appstoreconnect.NewInAppPurchaseRequest(appID, name, productID, connectmodel.InAppPurchaseTypeConsumable))
}

func UploadIPA(ctx context.Context, client *appstoreconnect.Client, appID, version, buildNumber, path string) (*connectmodel.BuildUploadResponse, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	upload, err := client.UploadBuild(ctx, appstoreconnect.NewBuildUploadRequest(appID, version, buildNumber, connectmodel.PlatformIos), []appstoreconnect.UploadFile{{Name: info.Name(), Size: info.Size(), Content: file}})
	if err != nil {
		return upload, err
	}
	return client.WaitForBuildUpload(ctx, upload.Data.ID, 10*time.Second)
}
