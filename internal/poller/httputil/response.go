package httputil

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// successResponse wraps data in the standard {"data": ...} envelope.
type successResponse struct {
	Data interface{} `json:"data"`
}

// listResponse wraps data with pagination metadata.
type listResponse struct {
	Data interface{} `json:"data"`
	Meta pageMeta    `json:"meta"`
}

type pageMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// errorBody is the standard error envelope.
type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// JSON writes a success response with the given status code.
func JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(successResponse{Data: data}); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}

// JSONList writes a paginated list response.
func JSONList(w http.ResponseWriter, data interface{}, page, pageSize int, total int64) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := listResponse{
		Data: data,
		Meta: pageMeta{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode JSON list response", "error", err)
	}
}

// Error writes an error response with the given status code, error code, and message.
func Error(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := errorResponse{
		Error: errorBody{
			Code:    code,
			Message: message,
		},
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode error response", "error", err)
	}
}
