//go:build !solution

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

func main() {
	wordcount := make(map[string]int)
	// fmt.Println("Start working")
	for _, file := range os.Args[1:] {
		f, err := os.Open(file)
		if err != nil {
			log.Fatal(err)
			return
		}
		// fmt.Println("File opened")
		defer f.Close()
		fileData, err := io.ReadAll(f)
		if err != nil {
			log.Fatal(err)
			return
		}
		// fmt.Println("File data read")
		for _, line := range strings.Split(string(fileData), "\n") {
			// for _, word := range strings.Split(line, " ") {
			// 	// if word == "" {
			// 	// 	continue
			// 	// }
			// 	// fmt.Println("Word:", word)
			// 	cnt, ok := wordcount[word]
			// 	if !ok {
			// 		wordcount[word] = 0
			// 	}
			// 	wordcount[word] = cnt + 1
			// }
			cnt, ok := wordcount[line]
			if !ok {
				wordcount[line] = 0
			}
			wordcount[line] = cnt + 1

		}
	}

	for word, cnt := range wordcount {
		if cnt >= 2 {
			fmt.Printf("%d\t%s\n", cnt, word)
		}
	}
}
