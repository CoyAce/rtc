package main

import (
	"bufio"
	"flag"
	"fmt"
	"live/rtc"
	"log"
	"os"
	"time"
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
		s := rtc.Server{}
		log.Fatal(s.ListenAndServe(*address))
	} else {
		fmt.Println("input sign:")
		var sign string
		fmt.Scanln(&sign)
		c := rtc.Client{ServerAddr: *address, Sign: rtc.Sign(sign), Retries: 3, Timeout: time.Second * 5}
		go func() {
			c.ListenAndServe("127.0.0.1:")
		}()

		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Println("input text:")
			text, _ := reader.ReadString('\n')
			text = text[:len(text)-1]
			c.SendText(text)
		}
	}
}
