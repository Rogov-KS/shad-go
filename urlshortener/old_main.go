//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
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

func RunServerWithRouting(port uint16) {
	router := func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("r.RequestURI = %s\n", r.RequestURI)
		initialPath := strings.Split(r.RequestURI, "/")[1]
		fmt.Printf("initialPath = %s\n", initialPath)
		switch {
		case r.RequestURI == "/pong":
			pongHandler(w, r)
		case r.RequestURI == "/shorten":
			shortenHandler(w, r)
		case strings.HasPrefix(r.RequestURI, "/go/"):
			goHandler(w, r)
		default:
			w.WriteHeader(404)
		}
	}
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), http.HandlerFunc(router))
	if err != nil {
		panic(err)
	}
}

func pongHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{"message": "pong"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func goHandler(w http.ResponseWriter, r *http.Request) {
	splittedUrl := strings.Split(r.RequestURI, "/")
	if len(splittedUrl) != 3 {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	reqUrl := splittedUrl[2]
	mutex.Lock()
	url, ok := keyToUrl[reqUrl]
	mutex.Unlock()
	if !ok {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	http.Redirect(w, r, url, http.StatusFound)
}

func shortenHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("Fail while reading Body: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	fmt.Printf("Read Body: %s\n", body)

	var req struct {
		URL string `json:"url"`
	}
	var resp struct {
		URL string `json:"url"`
		Key string `json:"key"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		fmt.Printf("Fail while parsing JSON: %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	fmt.Printf("Parsed URL: %s\n", req.URL)

	mutex.Lock()
	key, ok := urlToKey[req.URL]
	if !ok {
		counter++
		key = intToBase62(counter)
		urlToKey[req.URL] = key
		keyToUrl[key] = req.URL
	}
	mutex.Unlock()
	resp.URL = req.URL
	resp.Key = key
	w.Header().Set("Content-Type", "application/json")
	resBytes, err := json.Marshal(resp)
	_, _ = w.Write(resBytes)
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
	fmt.Printf("port: %d\n", port)
	return port, nil
}

func main() {
	port, err := GetPort()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	keyToUrl = make(map[string]string)
	urlToKey = make(map[string]string)
	RunServerWithRouting(port)
}
