package appstoreconnect

import "github.com/sunbeam27/app-store/appstoreconnect/model"

// NewInAppPurchaseRequest creates a draft consumable, non-consumable, or
// non-renewing subscription. Configure localization, availability and pricing
// separately; product creation does not submit it for review.
func NewInAppPurchaseRequest(appID, name, productID string, kind model.InAppPurchaseType) model.InAppPurchaseV2CreateRequest {
	var request model.InAppPurchaseV2CreateRequest
	request.Data.Type = "inAppPurchases"
	request.Data.Attributes = model.InAppPurchaseV2CreateRequestDataAttributes{Name: name, ProductID: productID, InAppPurchaseType: kind}
	request.Data.Relationships.App.Data = model.InAppPurchaseV2CreateRequestDataRelationshipsAppData{Type: "apps", ID: appID}
	return request
}

func NewSubscriptionGroupRequest(appID, referenceName string) model.SubscriptionGroupCreateRequest {
	var request model.SubscriptionGroupCreateRequest
	request.Data.Type = "subscriptionGroups"
	request.Data.Attributes.ReferenceName = referenceName
	request.Data.Relationships.App.Data = model.SubscriptionGroupCreateRequestDataRelationshipsAppData{Type: "apps", ID: appID}
	return request
}

func NewSubscriptionRequest(groupID, name, productID string) model.SubscriptionCreateRequest {
	var request model.SubscriptionCreateRequest
	request.Data.Type = "subscriptions"
	request.Data.Attributes.Name = name
	request.Data.Attributes.ProductID = productID
	request.Data.Relationships.Group.Data = model.SubscriptionCreateRequestDataRelationshipsGroupData{Type: "subscriptionGroups", ID: groupID}
	return request
}

func NewBuildUploadRequest(appID, version, buildNumber string, platform model.Platform) model.BuildUploadCreateRequest {
	var request model.BuildUploadCreateRequest
	request.Data.Type = "buildUploads"
	request.Data.Attributes = model.BuildUploadCreateRequestDataAttributes{CFBundleShortVersionString: version, CFBundleVersion: buildNumber, Platform: platform}
	request.Data.Relationships.App.Data = model.BuildUploadCreateRequestDataRelationshipsAppData{Type: "apps", ID: appID}
	return request
}

func NewBuildUploadFileRequest(uploadID, fileName string, size int64, assetType, uti string) model.BuildUploadFileCreateRequest {
	var request model.BuildUploadFileCreateRequest
	request.Data.Type = "buildUploadFiles"
	request.Data.Attributes = model.BuildUploadFileCreateRequestDataAttributes{FileName: fileName, FileSize: size, AssetType: assetType, UTI: uti}
	request.Data.Relationships.BuildUpload.Data = model.BuildUploadFileCreateRequestDataRelationshipsBuildUploadData{Type: "buildUploads", ID: uploadID}
	return request
}
