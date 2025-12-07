package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"rtc/core"
	"strconv"
)

var (
	address    = flag.String("a", "127.0.0.1:52000", "listen address")
	serverMode = flag.Bool("s", true, "server mode")
)

func main() {
	flag.Parse()
	fmt.Println("address:", *address)
	if *serverMode {
		fmt.Println("server mode")
		s := core.Server{}
		s.ListenAndServe(*address)
	} else {
		fmt.Println("input sign:")
		var sign string
		fmt.Scanln(&sign)

		uuid := "#" + strconv.Itoa(rand.Intn(90000)+10000)
		log.Println("client uuid:", uuid)
		c := core.Client{ServerAddr: *address, Status: make(chan struct{}), UUID: uuid, Sign: core.Sign(sign)}
		go func() {
			c.ListenAndServe("127.0.0.1:")
		}()
		c.Ready()

		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Println("input text:")
			text, _ := reader.ReadString('\n')
			text = text[:len(text)-1]
			c.SendText(text)
		}
	}
}
