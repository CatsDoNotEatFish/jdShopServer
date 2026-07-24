package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type APIResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func respondOK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, APIResponse{Code: 0, Message: "success", Data: data})
}

func respondMessage(w http.ResponseWriter, code int, message string) {
	httpStatus := codeToHTTPStatus(code)
	writeJSON(w, httpStatus, APIResponse{Code: code, Message: message, Data: nil})
}

func respondError(w http.ResponseWriter, code int, message string) {
	httpStatus := codeToHTTPStatus(code)
	writeJSON(w, httpStatus, APIResponse{Code: code, Message: message, Data: nil})
}

func respondPaginated(w http.ResponseWriter, items any, total int64, page, pageSize int) {
	respondOK(w, map[string]any{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func codeToHTTPStatus(code int) int {
	switch code {
	case 0:
		return http.StatusOK
	case 10001:
		return http.StatusBadRequest
	case 10002:
		return http.StatusUnauthorized
	case 10003:
		return http.StatusForbidden
	case 10004:
		return http.StatusNotFound
	case 10005:
		return http.StatusConflict
	case 10006:
		return http.StatusTooManyRequests
	case 10503:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func getQueryInt(r *http.Request, key string, defaultVal int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return defaultVal
	}
	return n
}

func getQueryInt64(r *http.Request, key string, defaultVal int64) int64 {
	v := r.URL.Query().Get(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return defaultVal
	}
	return n
}

func decodeBody(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}
