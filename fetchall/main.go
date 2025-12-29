//go:build !solution

package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

func fetch(url string, start_time time.Time, wg *sync.WaitGroup) {
	defer wg.Done()
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetch: %v\n", err)
		return
	}
	defer resp.Body.Close()
	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetch: reading %s: %v\n", url, err)
		return
	}
	end_time := time.Now()
	fmt.Printf("Time taken for url %s: %.2fs\n", url, end_time.Sub(start_time).Seconds())
}

func main() {
	start_time := time.Now()
	var wg sync.WaitGroup
	for _, url := range os.Args[1:] {
		wg.Add(1)
		go fetch(url, start_time, &wg)
	}
	wg.Wait()
}
