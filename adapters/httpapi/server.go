package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/steam9steam-prog/ez-vpn-lego/core/domain"
	"github.com/steam9steam-prog/ez-vpn-lego/core/ports"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/buildinfo"
)

const maxRequestBody = 64 << 10

type HealthChecker interface {
	Ping(context.Context) error
}

type Server struct {
	users     ports.UserService
	admins    ports.AdminService
	vpn       ports.VPNBootstrapService
	access    ports.AccessService
	health    HealthChecker
	tokenHash [32]byte
}

type userResponse struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Status    domain.LifecycleStatus `json:"status"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

func New(users ports.UserService, admins ports.AdminService, vpn ports.VPNBootstrapService, access ports.AccessService, health HealthChecker, apiToken string) *Server {
	return &Server{users: users, admins: admins, vpn: vpn, access: access, health: health, tokenHash: sha256.Sum256([]byte(apiToken))}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", s.getHealth)
	mux.Handle("POST /v1/bootstrap/owner", s.authenticateAdapter(http.HandlerFunc(s.bootstrapOwner)))
	mux.Handle("POST /v1/auth/telegram/pairing", s.authenticateAdmin(http.HandlerFunc(s.createTelegramPairing)))
	mux.Handle("POST /v1/auth/telegram/claim", s.authenticateAdapter(http.HandlerFunc(s.claimTelegramPairing)))
	mux.Handle("GET /v1/auth/telegram/resolve", s.authenticateAdapter(http.HandlerFunc(s.resolveTelegram)))
	mux.Handle("POST /v1/bootstrap/vpn", s.authenticateAdmin(http.HandlerFunc(s.bootstrapVPN)))
	mux.Handle("POST /v1/access", s.authenticateAdmin(http.HandlerFunc(s.createAccess)))
	mux.Handle("GET /v1/users", s.authenticateAdmin(http.HandlerFunc(s.listUsers)))
	mux.Handle("POST /v1/users", s.authenticateAdmin(http.HandlerFunc(s.createUser)))
	return s.recoverPanic(mux)
}

func (s *Server) createTelegramPairing(response http.ResponseWriter, request *http.Request) {
	result, err := s.admins.CreateTelegramPairing(request.Context(), request.Header.Get("X-Admin-ID"))
	if err != nil {
		writeError(response, http.StatusInternalServerError, "pairing_create_failed", "unable to create pairing link")
		return
	}
	writeJSON(response, http.StatusCreated, result)
}

func (s *Server) claimTelegramPairing(response http.ResponseWriter, request *http.Request) {
	var body struct {
		Token   string `json:"token"`
		Subject string `json:"subject"`
	}
	if err := decodeJSON(response, request, &body); err != nil {
		return
	}
	admin, err := s.admins.ClaimTelegramPairing(request.Context(), body.Token, body.Subject)
	if errors.Is(err, ports.ErrPairingTokenInvalid) || errors.Is(err, ports.ErrIdentityAlreadyBound) {
		writeError(response, http.StatusConflict, "pairing_rejected", err.Error())
		return
	}
	if err != nil {
		writeError(response, http.StatusInternalServerError, "pairing_failed", "unable to bind Telegram account")
		return
	}
	writeJSON(response, http.StatusOK, admin)
}

func (s *Server) resolveTelegram(response http.ResponseWriter, request *http.Request) {
	admin, err := s.admins.ResolveTelegram(request.Context(), request.URL.Query().Get("subject"))
	if errors.Is(err, ports.ErrUnauthorizedActor) {
		writeError(response, http.StatusNotFound, "identity_not_found", "Telegram account is not paired")
		return
	}
	if err != nil {
		writeError(response, http.StatusInternalServerError, "identity_resolve_failed", "unable to resolve Telegram account")
		return
	}
	writeJSON(response, http.StatusOK, admin)
}

func (s *Server) createAccess(response http.ResponseWriter, request *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(response, request, &body); err != nil {
		return
	}
	result, err := s.access.Create(request.Context(), ports.CreateAccessRequest{
		AdminID: request.Header.Get("X-Admin-ID"), Name: body.Name,
		IdempotencyKey: request.Header.Get("Idempotency-Key"),
	})
	if errors.Is(err, ports.ErrInvalidArgument) {
		writeError(response, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err != nil {
		writeError(response, http.StatusInternalServerError, "access_create_failed", "unable to create VPN access")
		return
	}
	writeJSON(response, http.StatusCreated, result)
}

func (s *Server) bootstrapVPN(response http.ResponseWriter, request *http.Request) {
	var body struct {
		PublicAddress string `json:"public_address"`
		Port          uint16 `json:"port"`
		Target        string `json:"target"`
		ServerName    string `json:"server_name"`
	}
	if err := decodeJSON(response, request, &body); err != nil {
		return
	}
	result, err := s.vpn.Bootstrap(request.Context(), ports.BootstrapVPNRequest{
		AdminID: request.Header.Get("X-Admin-ID"), PublicAddress: body.PublicAddress,
		Port: body.Port, Target: body.Target, ServerName: body.ServerName,
		IdempotencyKey: request.Header.Get("Idempotency-Key"),
	})
	if errors.Is(err, ports.ErrVPNAlreadyBootstrapped) {
		writeError(response, http.StatusConflict, "already_bootstrapped", err.Error())
		return
	}
	if errors.Is(err, ports.ErrUnauthorizedActor) {
		writeError(response, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	if errors.Is(err, ports.ErrInvalidArgument) {
		writeError(response, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err != nil {
		writeError(response, http.StatusInternalServerError, "bootstrap_failed", "VPN bootstrap failed and was rolled back")
		return
	}
	writeJSON(response, http.StatusCreated, result)
}

func (s *Server) bootstrapOwner(response http.ResponseWriter, request *http.Request) {
	admin, err := s.admins.BootstrapOwner(request.Context())
	if errors.Is(err, ports.ErrAlreadyBootstrapped) {
		writeError(response, http.StatusConflict, "already_bootstrapped", err.Error())
		return
	}
	if err != nil {
		writeError(response, http.StatusInternalServerError, "internal_error", "unable to bootstrap owner")
		return
	}
	writeJSON(response, http.StatusCreated, map[string]any{
		"id": admin.ID, "role": admin.Role, "status": admin.Status,
		"created_at": admin.CreatedAt, "updated_at": admin.UpdatedAt,
	})
}

func (s *Server) getHealth(response http.ResponseWriter, request *http.Request) {
	status := "healthy"
	code := http.StatusOK
	if err := s.health.Ping(request.Context()); err != nil {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}
	writeJSON(response, code, map[string]string{"status": status, "version": buildinfo.Version})
}

func (s *Server) listUsers(response http.ResponseWriter, request *http.Request) {
	users, err := s.users.List(request.Context())
	if err != nil {
		writeError(response, http.StatusInternalServerError, "internal_error", "unable to list users")
		return
	}
	result := make([]userResponse, 0, len(users))
	for _, user := range users {
		result = append(result, mapUser(user))
	}
	writeJSON(response, http.StatusOK, result)
}

func (s *Server) createUser(response http.ResponseWriter, request *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(response, request, &body); err != nil {
		return
	}
	result, err := s.users.Create(request.Context(), ports.CreateUserRequest{
		AdminID:        request.Header.Get("X-Admin-ID"),
		Name:           body.Name,
		IdempotencyKey: request.Header.Get("Idempotency-Key"),
	})
	if errors.Is(err, ports.ErrIdempotencyConflict) {
		writeError(response, http.StatusConflict, "idempotency_conflict", err.Error())
		return
	}
	if errors.Is(err, ports.ErrUnauthorizedActor) {
		writeError(response, http.StatusForbidden, "forbidden", "administrator is not active")
		return
	}
	if errors.Is(err, ports.ErrInvalidArgument) {
		writeError(response, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err != nil {
		writeError(response, http.StatusInternalServerError, "internal_error", "unable to create user")
		return
	}
	code := http.StatusCreated
	if result.Replayed {
		code = http.StatusOK
	}
	writeJSON(response, code, struct {
		OperationID string       `json:"operation_id"`
		User        userResponse `json:"user"`
		Replayed    bool         `json:"replayed"`
	}{OperationID: result.OperationID, User: mapUser(result.User), Replayed: result.Replayed})
}

func mapUser(user domain.User) userResponse {
	return userResponse{
		ID: user.ID, Name: user.Name, Status: user.Status,
		CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt,
	}
}

func (s *Server) authenticateAdapter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		provided := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
		providedHash := sha256.Sum256([]byte(provided))
		if provided == "" || subtle.ConstantTimeCompare(providedHash[:], s.tokenHash[:]) != 1 {
			writeError(response, http.StatusUnauthorized, "unauthorized", "valid bearer token required")
			return
		}
		next.ServeHTTP(response, request)
	})
}

func (s *Server) authenticateAdmin(next http.Handler) http.Handler {
	return s.authenticateAdapter(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Admin-ID") == "" {
			writeError(response, http.StatusUnauthorized, "administrator_required", "administrator identity required")
			return
		}
		next.ServeHTTP(response, request)
	}))
}

func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		defer func() {
			if recover() != nil {
				writeError(response, http.StatusInternalServerError, "internal_error", "internal server error")
			}
		}()
		next.ServeHTTP(response, request)
	})
}

func decodeJSON(response http.ResponseWriter, request *http.Request, target any) error {
	request.Body = http.MaxBytesReader(response, request.Body, maxRequestBody)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(response, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(response, http.StatusBadRequest, "invalid_json", "request body must contain one JSON object")
		return errors.New("request body contains trailing data")
	}
	return nil
}

func writeError(response http.ResponseWriter, status int, code string, message string) {
	writeJSON(response, status, map[string]string{"code": code, "message": message})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func TimeoutConfig() (readHeader, read, write, idle time.Duration) {
	return 5 * time.Second, 10 * time.Second, 30 * time.Second, 60 * time.Second
}
