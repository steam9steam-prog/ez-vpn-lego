package helper

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

type Server struct {
	engine    *Engine
	tokenHash [32]byte
}

func NewServer(engine *Engine, token string) *Server {
	return &Server{engine: engine, tokenHash: sha256.Sum256([]byte(token))}
}

func (server *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /v1/xray/apply", server.authenticate(http.HandlerFunc(server.applyXray)))
	return mux
}

func (server *Server) applyXray(response http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(response, request.Body, 16<<10)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input ApplyRequest
	if err := decoder.Decode(&input); err != nil {
		writeHelperError(response, http.StatusBadRequest, "invalid request")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeHelperError(response, http.StatusBadRequest, "trailing request data")
		return
	}
	if err := server.engine.Apply(request.Context(), input); err != nil {
		writeHelperError(response, http.StatusUnprocessableEntity, err.Error())
		return
	}
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte(`{"status":"applied"}` + "\n"))
}

func (server *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		provided := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
		providedHash := sha256.Sum256([]byte(provided))
		if provided == "" || subtle.ConstantTimeCompare(providedHash[:], server.tokenHash[:]) != 1 {
			writeHelperError(response, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(response, request)
	})
}

func writeHelperError(response http.ResponseWriter, status int, message string) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(map[string]string{"message": message})
}
