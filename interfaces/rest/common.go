package rest

import (
	"encoding/json"
	"net/http"

	"github.com/beeleelee/mall/domain/kernel"
)

func writeDomainError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")

	var code int
	switch {
	case kernel.IsNotFound(err):
		code = http.StatusNotFound
	case kernel.IsAlreadyExists(err):
		code = http.StatusConflict
	case kernel.IsInvalidArgument(err):
		code = http.StatusBadRequest
	case kernel.IsUnauthenticated(err):
		code = http.StatusUnauthorized
	case kernel.IsPermissionDenied(err):
		code = http.StatusForbidden
	default:
		code = http.StatusInternalServerError
	}

	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
