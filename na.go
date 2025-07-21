package main

import (
	"fmt"
	"unicode/utf8"
)

const word = "привет"

func main() {
	bword := []byte(word)
	fmt.Println(
		utf8.RuneCount(bword),
		len(word),
		len(string(word[1])),
	)
}
