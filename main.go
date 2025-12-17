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
	address         = flag.String("a", "0.0.0.0:52000", "listen address")
	config          = flag.String("c", "config.json", "config file")
	serverMode      = flag.Bool("s", false, "server mode")
	commandLineMode = flag.Bool("i", false, "server mode")
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

	// set uuid
	uuid := "#" + strconv.Itoa(rand.Intn(90000)+10000)
	log.Println("client uuid:", uuid)

	if *commandLineMode {
		// set sign
		fmt.Println("input sign:")
		var sign string
		fmt.Scanln(&sign)

		// setup client
		c := core.Client{ConfigName: *config, ServerAddr: *address, Status: make(chan struct{}), UUID: uuid, Sign: sign}
		go func() {
			c.ListenAndServe("0.0.0.0:")
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

	// setup client
	c := core.Load(*config)
	if c == nil {
		c = &core.Client{ConfigName: *config, ServerAddr: *address, Status: make(chan struct{}), UUID: uuid, Sign: "default"}
	} else {
		c.Status = make(chan struct{})
		c.ConfigName = *config
	}
	c.Store()
	go func() {
		c.ListenAndServe("0.0.0.0:")
	}()
	c.Ready()

	go func() {
		w := new(app.Window)
		w.Option(app.Title("rtc"))
		w.Option(app.Size(unit.Dp(463), unit.Dp(750)))
		w.Option(app.MinSize(unit.Dp(463)/2, unit.Dp(750)/2))
		if err := ui.Draw(w, c); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}
