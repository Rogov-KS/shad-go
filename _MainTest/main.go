package main

import "fmt"

func main() {
	fmt.Print("Hello world!")
	fmt.Printf("(1<<63-1)/10 = %s", ((1<<63 - 1) / 10))
}
