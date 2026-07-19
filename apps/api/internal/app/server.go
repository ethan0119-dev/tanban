package app

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethan0119-dev/tanban/apps/api/internal/cache"
	"github.com/ethan0119-dev/tanban/apps/api/internal/config"
	"github.com/ethan0119-dev/tanban/apps/api/internal/provider"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	DB                    *sql.DB
	Config                config.Config
	Logger                *slog.Logger
	Cache                 cache.Cache
	Payment               provider.PaymentProvider
	MockPayment           *provider.MockPayment
	Printer               provider.PrinterProvider
	AllowMockConfirmation bool
	publicRateMu          sync.Mutex
}

type envelope struct {
	Data  any       `json:"data,omitempty"`
	Meta  any       `json:"meta,omitempty"`
	Error *apiError `json:"error,omitempty"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func New(db *sql.DB, cfg config.Config, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	mockPayment := provider.NewMockPayment()
	var payment provider.PaymentProvider = mockPayment
	if cfg.PaymentProvider == "tianque" {
		payment = provider.TianQue{Config: provider.TianQueConfig{
			BaseURL: cfg.TianQue.BaseURL, OrgID: cfg.TianQue.OrgID,
			PrivateKey: cfg.TianQue.PrivateKey, PublicKey: cfg.TianQue.PublicKey,
			NotifyURL: cfg.TianQue.NotifyURL,
		}}
	}
	var printer provider.PrinterProvider = provider.MockPrinter{Logger: logger}
	if cfg.PrinterProvider == "xprinter" {
		printer = provider.XPrinter{}
	}
	return &Server{
		DB: db, Config: cfg, Logger: logger, Cache: cache.NewMemory(),
		Payment: payment, MockPayment: mockPayment, Printer: printer,
		AllowMockConfirmation: cfg.AllowMockConfirmation,
	}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP, middleware.Recoverer, middleware.Timeout(30*time.Second))
	r.Use(s.requestID, s.accessLog, s.cors)
	r.Get("/healthz", s.health)
	r.Get("/readyz", s.ready)
	r.Route("/api/v1", func(api chi.Router) {
		api.Post("/auth/login", s.login)
		api.Group(func(protected chi.Router) {
			protected.Use(s.authenticate)
			protected.Get("/auth/me", s.me)
			protected.Route("/platform", s.platformRoutes)
			protected.Route("/merchant", s.merchantRoutes)
		})
		api.Route("/public", s.publicRoutes)
		api.Post("/payments/tianque/callback", s.tianQueCallback)
		api.Post("/payments/mock/{providerOrderNo}/confirm", s.mockConfirm)
	})
	return r
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeData(w, http.StatusOK, map[string]any{"status": "ok", "service": "tanban-api"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.DB.PingContext(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "database is unavailable")
		return
	}
	writeData(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			b := make([]byte, 12)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			return
		}
		s.Logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(started).Milliseconds(), "request_id", requestID(r.Context()))
	})
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := false
		for _, candidate := range s.Config.CORSAllowedOrigins {
			if origin == candidate {
				allowed = true
				break
			}
		}
		if origin != "" && allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Request-ID")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions && origin != "" && !allowed {
			writeError(w, http.StatusForbidden, "CORS_ORIGIN_DENIED", "origin is not allowed")
			return
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeData(w http.ResponseWriter, status int, data any) {
	writeEnvelope(w, status, envelope{Data: data})
}

func writeList(w http.ResponseWriter, status int, data any, total, page, pageSize int) {
	writeEnvelope(w, status, envelope{Data: data, Meta: map[string]int{"total": total, "page": page, "page_size": pageSize}})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeEnvelope(w, status, envelope{Error: &apiError{Code: code, Message: message}})
}

func writeEnvelope(w http.ResponseWriter, status int, value envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return false
	}
	return true
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_ID", name+" must be a positive integer")
		return 0, false
	}
	return id, true
}

func pagination(r *http.Request) (page, pageSize, offset int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ = strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize, (page - 1) * pageSize
}

func validStatus(value string, allowed ...string) bool {
	value = strings.ToUpper(value)
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func handleSQLError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
		return
	}
	if strings.Contains(err.Error(), "1062") {
		writeError(w, http.StatusConflict, "ALREADY_EXISTS", "resource already exists")
		return
	}
	slog.Error("database operation failed", "error", err)
	writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "database operation failed")
}

type requestIDKey struct{}

func requestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey{}).(string)
	return value
}
