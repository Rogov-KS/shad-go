//go:build !solution

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
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

// Ключ для хранения logger в gin.Context
const loggerKey = "logger"
const scaleKey = "k"
const timeKey = "time"
const timeFormat = "15:04:05" // Формат времени: часы:минуты:секунды (24-часовой формат)

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
	router.GET("/", getDigClock)

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

func GetImage(c *gin.Context, scale int64, symbols []rune) *image.RGBA {
	logger := getLogger(c)

	splittedSym := strings.Split(Zero, "\n")
	symH := len(splittedSym)
	symW := len(splittedSym[0])

	splittedColon := strings.Split(Colon, "\n")
	colonH := len(splittedColon)
	colonW := len(splittedColon[0])

	realH := symH * int(scale)
	realW := (6*symW + 2*colonW) * int(scale)
	logger.Info("Cacl sym H and W", "width", symW, "height", symH, "colonW", colonW, "colonH", colonH)

	img := image.NewRGBA(image.Rect(0, 0, realW, realH))

	prevWidth := 0
	for _, sym := range symbols {
		logger.Info("Try to pain sym", "sym", sym)
		imgSym := int2Syms[sym]
		imgSymSplitted := strings.Split(imgSym, "\n")
		var curW int
		if sym == ':' {
			curW = colonW
		} else {
			curW = symW
		}
		curH := symH
		for i := 0; i < curW; i++ {
			for j := 0; j < curH; j++ {
				curSymVal := imgSymSplitted[j][i]
				var clr color.Color
				if curSymVal == '.' {
					clr = White
				} else {
					clr = Cyan
				}
				for k_w := int64(0); k_w < scale; k_w++ {
					for k_h := int64(0); k_h < scale; k_h++ {
						img.Set(prevWidth+i*int(scale)+int(k_w),
							j*int(scale)+int(k_h),
							clr)
					}
				}
			}
		}
		prevWidth += curW * int(scale)
	}
	logger.Info("Have written an image")
	return img
}

func getDigClock(c *gin.Context) {
	logger := getLogger(c)
	logger.Info("GetDigitalClock Request", "method", c.Request.Method, "path", c.Request.URL.Path)
	scaleStr := c.Query(scaleKey)
	var scale int64
	if scaleStr == "" {
		scale = 1
	} else {
		s, err := strconv.ParseInt(scaleStr, 10, 16)
		scale = s
		if err != nil {
			logger.Warn("Got error while parsing k", "error", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad query parametr k"})
			return
		}
	}
	if scale < 1 || scale > 30 {
		logger.Warn("Got invalid scale param (k)", "scale", scale)
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad query parametr k"})
		return

	}
	logger.Info("scale retrieved", "scale", scale)

	timeStr := c.Query(timeKey)
	logger.Info("Get TimeStr from query params", "time", timeStr)
	if timeStr == "" {
		timeStr = time.Now().Format(timeFormat)
	} else {
		// Проверяем формат и корректность времени через time.Parse
		_, err := time.Parse(timeFormat, timeStr)
		if err != nil {
			logger.Warn("Got incorrect time format", "time", timeStr, "error", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad time format"})
			return
		}
	}
	if len(timeStr) != 8 {
		logger.Warn("Got incorrect time", "time", timeStr)
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad time format"})
		return
	}
	symbols := make([]rune, 0)
	for _, symbol := range timeStr {
		_, ok := int2Syms[symbol]
		if !ok {
			logger.Warn("Got incorrect time for sym", "time", timeStr, "sym", symbol)
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad time format"})
			return
		}
		symbols = append(symbols, symbol)
	}
	logger.Info("Get TimeStr", "timeStr", timeStr)

	// else {
	// 	s, err := strconv.ParseInt(scaleStr, 10, 16)
	// 	scale = s
	// 	if err != nil {
	// 		logger.Warn("Got error while parsing key", "error", err)
	// 		c.JSON(http.StatusBadRequest, gin.H{"error": "bad query parametr key"})
	// 		return
	// 	}
	// }
	// time=15:04:05

	img := GetImage(c, scale, symbols)

	// Кодируем изображение в буфер памяти
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		logger.Error("failed to encode image", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode image"})
		return
	}

	// Возвращаем изображение с правильным Content-Type
	c.Data(http.StatusOK, "image/png", buf.Bytes())
}

// func goHandler(c *gin.Context) {
// 	logger := getLogger(c)
// 	key := c.Param("key")

// 	logger.Info("redirect request", "method", c.Request.Method, "key", key)

// 	mutex.Lock()
// 	url, ok := keyToUrl[key]
// 	mutex.Unlock()

// 	if !ok {
// 		logger.Warn("key not found", "key", key)
// 		c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
// 		return
// 	}

// 	logger.Info("redirecting", "key", key, "url", url)
// 	c.Redirect(http.StatusFound, url)
// }

// func shortenHandler(c *gin.Context) {
// 	logger := getLogger(c)
// 	var req struct {
// 		URL string `json:"url" binding:"required"`
// 	}

// 	// Автоматическая десериализация и валидация через Gin
// 	if err := c.ShouldBindJSON(&req); err != nil {
// 		logger.Warn("failed to parse JSON", "error", err)
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
// 		return
// 	}

// 	logger.Info("parsed URL", "url", req.URL)

// 	mutex.Lock()
// 	key, ok := urlToKey[req.URL]
// 	if !ok {
// 		counter++
// 		key = intToBase62(counter)
// 		urlToKey[req.URL] = key
// 		keyToUrl[key] = req.URL
// 		logger.Info("new URL shortened", "url", req.URL, "key", key)
// 	} else {
// 		logger.Info("existing URL found", "url", req.URL, "key", key)
// 	}
// 	mutex.Unlock()

// 	// Автоматическая сериализация через Gin
// 	c.JSON(http.StatusOK, gin.H{
// 		"url": req.URL,
// 		"key": key,
// 	})
// }

func GetPort() (port uint16, err error) {
	portFlag := flag.Uint64("port", 0, "port number to listen on")
	flag.Parse()

	if *portFlag == 0 {
		return 0, fmt.Errorf("usage: %s -port <port_number>", os.Args[0])
	}

	if *portFlag > 65535 {
		return 0, fmt.Errorf("invalid port number: port must be between 1 and 65535")
	}

	port = uint16(*portFlag)
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
