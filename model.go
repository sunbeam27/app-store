package app_store

// --------- 定义与 app_store 交互的数据结构 ----------
import (
	"errors"
	"fmt"
)

type ErrorResponse struct {
	ErrorCode    int    `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

func (e ErrorResponse) Error() string {
	return fmt.Sprintf("error code: %d, error message: %s", e.ErrorCode, e.ErrorMessage)
}

func UnwrapError(err error) *ErrorResponse {
	var e *ErrorResponse
	if errors.As(err, &e) {
		return e
	}
	return nil
}
