package appstoreconnect

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sunbeam27/app-store/appstoreconnect/model"
)

type UploadFile struct {
	Name    string
	Size    int64
	Content io.ReaderAt
	// AssetType defaults to ASSET. UTI defaults from .ipa or .pkg extensions.
	AssetType string
	UTI       string
	// Checksums is optional, as in Apple's BuildUploadFileUpdateRequest.
	Checksums *model.Checksums
}

type UploadError struct {
	UploadID string
	FileID   string
	Err      error
}

func (e *UploadError) Error() string {
	return fmt.Sprintf("build upload %s, file %s: %v", e.UploadID, e.FileID, e.Err)
}
func (e *UploadError) Unwrap() error { return e.Err }

func (file UploadFile) prepare() (UploadFile, error) {
	if file.Content == nil || file.Name == "" || file.Size < 1 || file.Size > 9007199254740991 {
		return file, errors.New("file content, name, and size in Apple's supported range are required")
	}
	if file.AssetType == "" {
		file.AssetType = "ASSET"
	}
	if file.AssetType != "ASSET" && file.AssetType != "ASSET_DESCRIPTION" && file.AssetType != "ASSET_SPI" {
		return file, errors.New("invalid build asset type")
	}
	if file.UTI == "" {
		switch strings.ToLower(filepath.Ext(file.Name)) {
		case ".ipa":
			file.UTI = "com.apple.ipa"
		case ".pkg":
			file.UTI = "com.apple.pkg"
		default:
			return file, errors.New("specify a UTI for files other than .ipa or .pkg")
		}
	}
	switch file.UTI {
	case "com.apple.binary-property-list", "com.apple.ipa", "com.apple.pkg", "com.apple.xml-property-list", "com.pkware.zip-archive":
	default:
		return file, errors.New("invalid build file UTI")
	}
	return file, nil
}

// UploadBuild reserves the build and uploads every file without loading whole
// files into memory. It returns the upload even on later failure, for recovery.
// A successful return means files were committed; processing is asynchronous.
func (c *Client) UploadBuild(ctx context.Context, request model.BuildUploadCreateRequest, files []UploadFile) (*model.BuildUploadResponse, error) {
	if len(files) == 0 {
		return nil, errors.New("at least one build file is required")
	}
	prepared := make([]UploadFile, len(files))
	for i, file := range files {
		var err error
		prepared[i], err = file.prepare()
		if err != nil {
			return nil, err
		}
	}
	if request.Data.Relationships.App.Data.ID == "" || request.Data.Attributes.CFBundleVersion == "" || request.Data.Attributes.CFBundleShortVersionString == "" || request.Data.Attributes.Platform == "" {
		return nil, errors.New("app ID, version, build number and platform are required")
	}
	upload, err := c.CreateBuildUpload(ctx, request)
	if err != nil {
		return nil, err
	}
	if upload.Data.ID == "" {
		return upload, errors.New("Apple returned a build upload without an ID")
	}
	for _, file := range prepared {
		if _, err := c.UploadBuildFile(ctx, upload.Data.ID, file); err != nil {
			return upload, err
		}
	}
	response, err := c.GetBuildUpload(ctx, upload.Data.ID, nil)
	if err != nil {
		return upload, &UploadError{UploadID: upload.Data.ID, Err: err}
	}
	return response, nil
}

// UploadBuildFile reserves, streams and commits one file on an existing upload.
func (c *Client) UploadBuildFile(ctx context.Context, uploadID string, file UploadFile) (*model.BuildUploadFileResponse, error) {
	file, err := file.prepare()
	if err != nil {
		return nil, err
	}
	if uploadID == "" {
		return nil, errors.New("build upload ID is required")
	}
	reservation, err := c.CreateBuildUploadFile(ctx, NewBuildUploadFileRequest(uploadID, file.Name, file.Size, file.AssetType, file.UTI))
	if err != nil {
		return nil, &UploadError{UploadID: uploadID, Err: err}
	}
	fileID := reservation.Data.ID
	if fileID == "" || reservation.Data.Attributes == nil {
		return reservation, &UploadError{UploadID: uploadID, FileID: fileID, Err: errors.New("Apple returned an incomplete file reservation")}
	}
	if err := c.UploadOperations(ctx, file.Content, file.Size, reservation.Data.Attributes.UploadOperations); err != nil {
		return reservation, &UploadError{UploadID: uploadID, FileID: fileID, Err: err}
	}
	request := model.BuildUploadFileUpdateRequest{Data: model.BuildUploadFileUpdateRequestData{Type: "buildUploadFiles", ID: fileID, Attributes: &model.BuildUploadFileUpdateRequestDataAttributes{Uploaded: model.Ptr(true), SourceFileChecksums: file.Checksums}}}
	committed, err := c.CommitBuildUploadFile(ctx, fileID, request)
	if err != nil {
		return reservation, &UploadError{UploadID: uploadID, FileID: fileID, Err: err}
	}
	return committed, nil
}

// UploadOperations executes Apple's complete upload plan. It sends only the
// supplied operation headers, never this client's API bearer token. Retrying
// these operations is safe before committing the reservation.
func (c *Client) UploadOperations(ctx context.Context, content io.ReaderAt, size int64, operations []model.DeliveryFileUploadOperation) error {
	if content == nil || size <= 0 || len(operations) == 0 {
		return errors.New("file content and upload operations are required")
	}
	plan := append([]model.DeliveryFileUploadOperation(nil), operations...)
	sort.Slice(plan, func(i, j int) bool { return optional(plan[i].Offset) < optional(plan[j].Offset) })
	var offset int64
	for _, op := range plan {
		if op.Offset == nil || op.Length == nil || *op.Offset != offset || *op.Length <= 0 || *op.Length > size-offset {
			return errors.New("upload operations must cover the file exactly, without gaps or overlaps")
		}
		if op.Method == nil || *op.Method != "PUT" {
			return errors.New("unsupported upload operation method")
		}
		u, err := url.Parse(optional(op.URL))
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
			return errors.New("upload operation requires an HTTPS URL")
		}
		offset += *op.Length
	}
	if offset != size {
		return errors.New("upload operations do not cover the complete file")
	}
	for i, op := range operations {
		if err := ctx.Err(); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, *op.Method, *op.URL, io.NewSectionReader(content, *op.Offset, *op.Length))
		if err != nil {
			return err
		}
		req.ContentLength = *op.Length
		for _, header := range op.RequestHeaders {
			req.Header.Set(optional(header.Name), optional(header.Value))
		}
		resp, err := c.transport.HTTP.Do(req)
		if err != nil {
			return fmt.Errorf("upload part %d: %w", i, err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("upload part %d: %w", i, readErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return &APIError{HTTPStatusCode: resp.StatusCode, Headers: resp.Header.Clone(), Body: body, ErrorMessage: fmt.Sprintf("upload part %d failed", i)}
		}
	}
	return nil
}

func optional[T any](value *T) T {
	if value != nil {
		return *value
	}
	var zero T
	return zero
}

type BuildProcessingError struct {
	UploadID string
	State    *model.BuildUploadAttributesState
}

func (e *BuildProcessingError) Error() string {
	return "build upload " + e.UploadID + " failed processing"
}

// WaitForBuildUpload waits for Apple's processing result. Cancel or set a
// deadline on ctx to bound the wait. Warnings remain available in the response.
func (c *Client) WaitForBuildUpload(ctx context.Context, uploadID string, interval time.Duration) (*model.BuildUploadResponse, error) {
	if uploadID == "" {
		return nil, errors.New("build upload ID is required")
	}
	if interval <= 0 {
		interval = 10 * time.Second
	}
	for {
		response, err := c.GetBuildUpload(ctx, uploadID, nil)
		if err != nil {
			return nil, err
		}
		if response.Data.Attributes == nil || response.Data.Attributes.State == nil || response.Data.Attributes.State.State == nil {
			return response, errors.New("Apple returned no build processing state")
		}
		state := response.Data.Attributes.State
		switch *state.State {
		case model.BuildUploadStateComplete:
			return response, nil
		case model.BuildUploadStateFailed:
			return response, &BuildProcessingError{UploadID: uploadID, State: state}
		case model.BuildUploadStateAwaitingUpload, model.BuildUploadStateProcessing:
		default:
			return response, fmt.Errorf("unrecognized build upload state: %s", *state.State)
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return response, ctx.Err()
		case <-timer.C:
		}
	}
}
