//go:build !solution

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v2"
)

// Config представляет конфигурацию файрвола
type Config struct {
	Rules []Rule `yaml:"rules"`
}

// Rule представляет одно правило файрвола
type Rule struct {
	Endpoint               string   `yaml:"endpoint"`
	ForbiddenUserAgents    []string `yaml:"forbidden_user_agents"`
	ForbiddenHeaders       []string `yaml:"forbidden_headers"`
	RequiredHeaders        []string `yaml:"required_headers"`
	MaxRequestLengthBytes  *int     `yaml:"max_request_length_bytes"`
	MaxResponseLengthBytes *int     `yaml:"max_response_length_bytes"`
	ForbiddenResponseCodes []int    `yaml:"forbidden_response_codes"`
	ForbiddenRequestRE     []string `yaml:"forbidden_request_re"`
	ForbiddenResponseRE    []string `yaml:"forbidden_response_re"`
}

// loadConfig загружает конфигурацию из YAML файла
func loadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	// Если файл пустой, возвращаем пустую конфигурацию
	if len(data) == 0 {
		return &config, nil
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &config, nil
}

// FirewallTransport реализует http.RoundTripper для фильтрации запросов
type FirewallTransport struct {
	config *Config
	logger *slog.Logger
}

func (f *FirewallTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Находим правила для endpoint
	rules := f.getRulesForEndpoint(req.URL.Path)

	// Читаем тело запроса для проверки
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err == nil {
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			req.ContentLength = int64(len(bodyBytes))
		} else {
			req.Body = io.NopCloser(bytes.NewBuffer(nil))
			req.ContentLength = 0
		}
	}

	// Проверяем запрос по правилам
	if f.shouldBlockRequest(req, string(bodyBytes), rules) {
		f.logger.Info("request blocked", "path", req.URL.Path)
		return &http.Response{
			StatusCode:    http.StatusForbidden,
			Status:        "403 Forbidden",
			Proto:         req.Proto,
			ProtoMajor:    req.ProtoMajor,
			ProtoMinor:    req.ProtoMinor,
			Header:        http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
			Body:          io.NopCloser(strings.NewReader("Forbidden")),
			ContentLength: int64(len("Forbidden")),
			Request:       req,
		}, nil
	}

	// Выполняем запрос
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Читаем тело ответа для проверки
	var respBytes []byte
	if resp.Body != nil {
		respBytes, err = io.ReadAll(resp.Body)
		if err == nil {
			resp.Body = io.NopCloser(bytes.NewBuffer(respBytes))
			resp.ContentLength = int64(len(respBytes))
		}
	}

	// Проверяем ответ по правилам
	if f.shouldBlockResponse(resp, string(respBytes), rules) {
		f.logger.Info("response blocked", "path", req.URL.Path)
		resp.Body.Close()
		return &http.Response{
			StatusCode:    http.StatusForbidden,
			Status:        "403 Forbidden",
			Proto:         req.Proto,
			ProtoMajor:    req.ProtoMajor,
			ProtoMinor:    req.ProtoMinor,
			Header:        http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
			Body:          io.NopCloser(strings.NewReader("Forbidden")),
			ContentLength: int64(len("Forbidden")),
			Request:       req,
		}, nil
	}

	return resp, nil
}

func (f *FirewallTransport) getRulesForEndpoint(path string) []Rule {
	if f.config == nil {
		return nil
	}
	var matchingRules []Rule
	for _, rule := range f.config.Rules {
		if rule.Endpoint == path {
			matchingRules = append(matchingRules, rule)
		}
	}
	return matchingRules
}

func (f *FirewallTransport) shouldBlockRequest(req *http.Request, body string, rules []Rule) bool {
	for _, rule := range rules {
		// Проверка forbidden_user_agents
		userAgent := req.UserAgent()
		for _, pattern := range rule.ForbiddenUserAgents {
			matched, err := regexp.MatchString(pattern, userAgent)
			if err == nil && matched {
				return true
			}
		}

		// Проверка forbidden_headers
		for _, headerPattern := range rule.ForbiddenHeaders {
			parts := strings.SplitN(headerPattern, ":", 2)
			if len(parts) == 2 {
				headerName := strings.TrimSpace(parts[0])
				headerValuePattern := strings.TrimSpace(parts[1])
				headerValue := req.Header.Get(headerName)
				matched, err := regexp.MatchString(headerValuePattern, headerValue)
				if err == nil && matched {
					return true
				}
			}
		}

		// Проверка required_headers
		for _, requiredHeader := range rule.RequiredHeaders {
			if req.Header.Get(requiredHeader) == "" {
				return true
			}
		}

		// Проверка max_request_length_bytes
		if rule.MaxRequestLengthBytes != nil {
			if len(body) > *rule.MaxRequestLengthBytes {
				return true
			}
		}

		// Проверка forbidden_request_re
		for _, pattern := range rule.ForbiddenRequestRE {
			matched, err := regexp.MatchString(pattern, body)
			if err == nil && matched {
				return true
			}
		}
	}

	return false
}

func (f *FirewallTransport) shouldBlockResponse(resp *http.Response, body string, rules []Rule) bool {
	for _, rule := range rules {
		// Проверка forbidden_response_codes
		for _, forbiddenCode := range rule.ForbiddenResponseCodes {
			if resp.StatusCode == forbiddenCode {
				return true
			}
		}

		// Проверка max_response_length_bytes
		if rule.MaxResponseLengthBytes != nil {
			if len(body) > *rule.MaxResponseLengthBytes {
				return true
			}
		}

		// Проверка forbidden_response_re
		for _, pattern := range rule.ForbiddenResponseRE {
			matched, err := regexp.MatchString(pattern, body)
			if err == nil && matched {
				return true
			}
		}
	}

	return false
}

// setupGracefulShutdown настраивает graceful shutdown для HTTP сервера
func setupGracefulShutdown(srv *http.Server, logger *slog.Logger) {
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		logger.Info("shutting down server gracefully")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("server shutdown error", "error", err)
		} else {
			logger.Info("server stopped")
		}
	}()
}

func main() {
	// Парсинг аргументов командной строки
	serviceAddr := flag.String("service-addr", "", "адрес защищаемого сервиса")
	confPath := flag.String("conf", "", "путь к .yaml конфигу с правилами")
	addr := flag.String("addr", "", "адрес, на котором будет развёрнут файрвол")
	flag.Parse()

	// Инициализация logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Валидация аргументов
	if *serviceAddr == "" {
		log.Fatal("необходимо указать -service-addr")
	}
	if *confPath == "" {
		log.Fatal("необходимо указать -conf")
	}
	if *addr == "" {
		log.Fatal("необходимо указать -addr")
	}

	// Парсинг адреса целевого сервиса
	targetURL, err := url.Parse(*serviceAddr)
	if err != nil {
		logger.Error("invalid service address", "error", err, "addr", *serviceAddr)
		log.Fatal(err)
	}

	// Загрузка конфигурации
	config, err := loadConfig(*confPath)
	if err != nil {
		logger.Error("failed to load config", "error", err, "path", *confPath)
		log.Fatal(err)
	}

	// Создание reverse proxy с кастомным Transport
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.Transport = &FirewallTransport{
		config: config,
		logger: logger,
	}

	// Создание HTTP сервера
	srv := &http.Server{
		Addr:         *addr,
		Handler:      proxy,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Настройка graceful shutdown
	setupGracefulShutdown(srv, logger)

	logger.Info("starting firewall server",
		"addr", *addr,
		"service-addr", *serviceAddr,
		"config", *confPath,
	)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server failed", "error", err)
		log.Fatal(err)
	}
}
