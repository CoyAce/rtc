package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"rtc/internal/audio"
	"rtc/ui"
	"rtc/ui/native"
	"rtc/ui/view"
	"strconv"
	"time"

	"gioui.org/x/explorer"
	"github.com/CoyAce/wi"

	"gioui.org/app"
	_ "gioui.org/app/permission/microphone"
	_ "gioui.org/app/permission/storage"
	"gioui.org/unit"
)

var (
	address          = flag.String("a", "0.0.0.0:52000", "listen address")
	config           = flag.String("c", "config.json", "config file")
	testAudioLatency = flag.Bool("t", false, "test audio latency")
)

func main() {
	flag.Parse()
	if *testAudioLatency {
		audio.TestAudioLatency()
	}
	fmt.Println("address:", *address)

	// set uuid
	uuid := "#" + strconv.Itoa(rand.Intn(90000)+10000)
	log.Println("client uuid:", uuid)

	go triggerNetworkPermission()
	go func() {
		w := new(app.Window)
		w.Option(app.Title("◯"))
		w.Option(app.Size(unit.Dp(463), unit.Dp(750)))
		w.Option(app.MinSize(unit.Dp(463)/1.5, unit.Dp(750)/1.5))
		initTools(w)
		// setup client
		c := setup(uuid)
		c.Store()
		go func() {
			c.ListenAndServe("0.0.0.0:")
		}()
		c.Ready()
		if err := ui.Draw(w, c); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func setup(uuid string) *wi.Client {
	c := wi.Load(view.GetConfig(*config))
	if c == nil {
		c = &wi.Client{
			Config:   wi.Config{ServerAddr: *address},
			Identity: wi.Identity{UUID: uuid, Sign: "default"},
		}
	}
	c.Status = make(chan struct{})
	c.DataDir = view.GetDataDir()
	c.ExternalDir = view.GetExternalDir()
	c.ConfigName = *config
	c.SyncFunc = view.SyncIcon
	// save client to global pointer
	wi.DefaultClient = c
	wi.Mkdir(view.GetDir(c.FullID()))
	return c
}

func initTools(window *app.Window) {
	view.Picker = explorer.NewExplorer(window)
	native.Tool = native.NewPlatformTool(window)
}

func triggerNetworkPermission() {
	time.Sleep(500 * time.Millisecond)
	resp, err := http.Head("https://www.apple.com")
	if err != nil {
		log.Printf("HEAD request failed: %v", err)
		return
	}
	defer resp.Body.Close()
}
