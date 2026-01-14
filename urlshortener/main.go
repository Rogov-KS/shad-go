//go:build !solution

package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	keyToUrl map[string]string // key -> URL
	urlToKey map[string]string // URL -> key (для проверки дубликатов)
	counter  int64
	mutex    sync.Mutex
)

const base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func intToBase62(n int64) string {
	if n == 0 {
		return "0"
	}
	var result []byte
	for n > 0 {
		result = append([]byte{base62Chars[n%62]}, result...)
		n /= 62
	}
	return string(result)
}

func generateKey() string {
	mutex.Lock()
	defer mutex.Unlock()
	counter++
	return intToBase62(counter)
}

// Ключ для хранения logger в gin.Context
const loggerKey = "logger"

// slogMiddleware для логирования запросов через slog и установки logger в контекст
func slogMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Устанавливаем logger в контекст для использования в handlers
		c.Set(loggerKey, logger)

		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// Обрабатываем запрос
		c.Next()

		// Логируем после обработки
		latency := time.Since(start)
		status := c.Writer.Status()

		logger.Info("request processed",
			"method", method,
			"path", path,
			"status", status,
			"latency_ms", latency.Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}

// recoveryMiddleware обрабатывает паники и возвращает 500 ошибку
func recoveryMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		// Получаем logger из контекста, если есть, иначе используем переданный
		if ctxLogger, exists := c.Get(loggerKey); exists {
			if l, ok := ctxLogger.(*slog.Logger); ok {
				l.Error("panic recovered",
					"error", recovered,
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
				)
			}
		} else {
			logger.Error("panic recovered",
				"error", recovered,
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
			)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	})
}

// getLogger извлекает logger из gin.Context
func getLogger(c *gin.Context) *slog.Logger {
	if logger, exists := c.Get(loggerKey); exists {
		if l, ok := logger.(*slog.Logger); ok {
			return l
		}
	}
	// Fallback на глобальный logger, если не найден в контексте
	return slog.Default()
}

func RunServerWithRouting(port uint16) {
	// Инициализация структурированного логирования
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Создание роутера с помощью Gin
	// gin.New() создает роутер без middleware по умолчанию
	router := gin.New()

	// Применение middleware
	// Recovery middleware должен быть первым
	router.Use(recoveryMiddleware(logger))
	// Логирование запросов через slog
	router.Use(slogMiddleware(logger))

	// Регистрация маршрутов
	router.GET("/pong", pongHandler)
	router.POST("/shorten", shortenHandler)
	router.GET("/go/:key", goHandler)

	// Создание HTTP сервера
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
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

	logger.Info("starting server", "port", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server failed", "error", err)
		log.Fatal(err)
	}
}

func pongHandler(c *gin.Context) {
	logger := getLogger(c)
	logger.Info("pong request", "method", c.Request.Method, "path", c.Request.URL.Path)
	c.JSON(http.StatusOK, gin.H{"message": "pong"})
}

func goHandler(c *gin.Context) {
	logger := getLogger(c)
	key := c.Param("key")

	logger.Info("redirect request", "method", c.Request.Method, "key", key)

	mutex.Lock()
	url, ok := keyToUrl[key]
	mutex.Unlock()

	if !ok {
		logger.Warn("key not found", "key", key)
		c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
		return
	}

	logger.Info("redirecting", "key", key, "url", url)
	c.Redirect(http.StatusFound, url)
}

func shortenHandler(c *gin.Context) {
	logger := getLogger(c)
	var req struct {
		URL string `json:"url" binding:"required"`
	}

	// Автоматическая десериализация и валидация через Gin
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warn("failed to parse JSON", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	logger.Info("parsed URL", "url", req.URL)

	mutex.Lock()
	key, ok := urlToKey[req.URL]
	if !ok {
		counter++
		key = intToBase62(counter)
		urlToKey[req.URL] = key
		keyToUrl[key] = req.URL
		logger.Info("new URL shortened", "url", req.URL, "key", key)
	} else {
		logger.Info("existing URL found", "url", req.URL, "key", key)
	}
	mutex.Unlock()

	// Автоматическая сериализация через Gin
	c.JSON(http.StatusOK, gin.H{
		"url": req.URL,
		"key": key,
	})
}

func GetPort() (port uint16, err error) {
	args := os.Args
	if len(args) != 3 || args[1] != "-port" {
		return 0, fmt.Errorf("usage: %s -port <port_number>", args[0])
	}
	// В Go нельзя напрямую кастовать строку в число
	// Нужно использовать strconv.ParseInt или strconv.Atoi
	portInt, err := strconv.ParseUint(args[2], 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid port number: %w", err)
	}
	port = uint16(portInt) // Каст uint64 -> uint16 (числовые типы можно кастовать)
	slog.Info("parsed port", "port", port)
	return port, nil
}

func main() {
	port, err := GetPort()
	if err != nil {
		slog.Error("failed to get port", "error", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	keyToUrl = make(map[string]string)
	urlToKey = make(map[string]string)
	RunServerWithRouting(port)
}
