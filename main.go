package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"rtc/core"
	"rtc/ui"
	"strconv"

	"gioui.org/app"
	"gioui.org/unit"
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
		log.Fatal(s.ListenAndServe(*address))
		return
	}

	go func() {
		w := new(app.Window)
		w.Option(app.Title("rtc"))
		w.Option(app.Size(unit.Dp(463), unit.Dp(750)))
		if err := ui.Draw(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()

	// set sign
	fmt.Println("input sign:")
	var sign string
	fmt.Scanln(&sign)

	// set uuid
	uuid := "#" + strconv.Itoa(rand.Intn(90000)+10000)
	log.Println("client uuid:", uuid)

	// setup client
	c := core.Client{ServerAddr: *address, Status: make(chan struct{}), UUID: uuid, Sign: core.Sign(sign)}
	go func() {
		c.ListenAndServe("127.0.0.1:")
	}()
	c.Ready()

	// send text
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("input text:")
		text, _ := reader.ReadString('\n')
		text = text[:len(text)-1]
		c.SendText(text)
	}
}
