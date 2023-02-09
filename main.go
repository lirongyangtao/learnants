package main

import (
	"fmt"
	"time"

	"learnants/pool"
)

func main() {
	p := pool.NewPool(10)
	for i := 0; i < 7; i++ {
		err := p.Submit(func() {
			fmt.Println("hello world", i)
		})
		if err != nil {
			panic(err)
		}
	}
	p.Submit(func() {
		fmt.Println("hello world")
	})
	time.Sleep(1 * time.Minute)
}
