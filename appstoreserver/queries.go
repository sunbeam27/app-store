package appstoreserver

import (
	"context"
	"errors"
	"net/url"
	"strconv"

	"github.com/sunbeam27/app-store/appstoreserver/model"
)

type GetTransactionHistoryVersion string

const (
	GetTransactionHistoryVersionV1 GetTransactionHistoryVersion = "v1"
	GetTransactionHistoryVersionV2 GetTransactionHistoryVersion = "v2"
)

func (c *AppStoreServerAPIClient) GetAllSubscriptionStatuses(ctx context.Context, transactionID string, statuses []model.Status) (*model.StatusResponse, error) {
	query := url.Values{}
	for _, status := range statuses {
		query.Add("status", strconv.FormatInt(int64(status), 10))
	}
	var response model.StatusResponse
	if err := c.call(ctx, "GET", "/inApps/v1/subscriptions/"+url.PathEscape(transactionID), query, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *AppStoreServerAPIClient) GetRefundHistory(ctx context.Context, transactionID, revision string) (*model.RefundHistoryResponse, error) {
	query := url.Values{}
	if revision != "" {
		query.Set("revision", revision)
	}
	var response model.RefundHistoryResponse
	if err := c.call(ctx, "GET", "/inApps/v2/refund/lookup/"+url.PathEscape(transactionID), query, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *AppStoreServerAPIClient) GetNotificationHistory(ctx context.Context, paginationToken string, request model.NotificationHistoryRequest) (*model.NotificationHistoryResponse, error) {
	query := url.Values{}
	if paginationToken != "" {
		query.Set("paginationToken", paginationToken)
	}
	var response model.NotificationHistoryResponse
	if err := c.call(ctx, "POST", "/inApps/v1/notifications/history", query, request, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetTransactionHistory defaults to V1, matching Apple's overload. Pass V2 for
// current transaction history. Keep the same filters on subsequent revisions.
func (c *AppStoreServerAPIClient) GetTransactionHistory(ctx context.Context, transactionID, revision string, request *model.TransactionHistoryRequest, versions ...GetTransactionHistoryVersion) (*model.HistoryResponse, error) {
	version := GetTransactionHistoryVersionV1
	if len(versions) > 1 {
		return nil, errors.New("provide at most one history version")
	}
	if len(versions) == 1 {
		version = versions[0]
	}
	if version != GetTransactionHistoryVersionV1 && version != GetTransactionHistoryVersionV2 {
		return nil, errors.New("invalid history version")
	}
	query := url.Values{}
	if revision != "" {
		query.Set("revision", revision)
	}
	if request != nil {
		if request.StartDate != nil {
			query.Set("startDate", strconv.FormatInt(*request.StartDate, 10))
		}
		if request.EndDate != nil {
			query.Set("endDate", strconv.FormatInt(*request.EndDate, 10))
		}
		for _, v := range request.ProductIDs {
			query.Add("productId", v)
		}
		for _, v := range request.ProductTypes {
			query.Add("productType", string(v))
		}
		for _, v := range request.SubscriptionGroupIdentifiers {
			query.Add("subscriptionGroupIdentifier", v)
		}
		if request.Sort != nil {
			query.Set("sort", string(*request.Sort))
		}
		if request.InAppOwnershipType != nil {
			query.Set("inAppOwnershipType", string(*request.InAppOwnershipType))
		}
		if request.Revoked != nil {
			query.Set("revoked", strconv.FormatBool(*request.Revoked))
		}
	}
	var response model.HistoryResponse
	if err := c.call(ctx, "GET", "/inApps/"+string(version)+"/history/"+url.PathEscape(transactionID), query, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *AppStoreServerAPIClient) UploadImage(ctx context.Context, imageID string, image []byte, sizes ...model.ImageSize) error {
	if len(sizes) > 1 {
		return errors.New("provide at most one image size")
	}
	query := url.Values{}
	if len(sizes) == 1 {
		query.Set("imageSize", string(sizes[0]))
	}
	return c.transport.Do(ctx, "PUT", "/inApps/v1/messaging/image/"+url.PathEscape(imageID), query, image, "image/png", nil)
}
