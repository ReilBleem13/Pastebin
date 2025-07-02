package main

import "fmt"

func main() {
	var firstNum int
	if _, err := fmt.Scanf("%d", &firstNum); err != nil {
		fmt.Printf("failed: %v", err)
	}
	fmt.Printf("Первое число: %d", firstNum)
}
