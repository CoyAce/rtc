package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/CoyAce/live/rtc"
)

var (
	address    = flag.String("a", "127.0.0.1:52000", "listen address")
	serverMode = flag.Bool("s", true, "server mode")
)

func main() {
	flag.Parse()
	if *serverMode {

	}
}
