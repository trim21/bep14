package bep14

import (
	"fmt"
	"sync"
)

func Example() {
	c := New(6007, EnableV6(), EnableV4())

	var g sync.WaitGroup

	g.Add(1)
	go func() {
		defer g.Done()
		c.Start()
	}()

	g.Add(1)
	go func() {
		defer g.Done()
		for ih := range c.C {
			fmt.Println(ih)
		}
	}()

	c.Announce([]string{"88e17659cc7f6b94d8c844d02413471253c1bf49"})

	g.Wait()
}
