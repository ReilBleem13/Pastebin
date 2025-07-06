package main

import "fmt"

func add_withoutPointer(a int) {
	a += 2
}

func add_withPointer(a *int) {
	*a += 2
}

func main() {
	b := 4
	fmt.Printf("До сложения:%d\n", b)
	add_withoutPointer(b)
	fmt.Printf("После сложения:%d\n", b)

	c := 4
	fmt.Printf("До сложения:%d\n", c)
	add_withPointer(&c)
	fmt.Printf("После сложения:%d\n", c)
}
