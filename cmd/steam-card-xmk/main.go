// Command steam-card-xmk farms Steam trading cards by phantom-launching games.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/XOMAK2245/steam-card-xmk/internal/config"
	"github.com/XOMAK2245/steam-card-xmk/internal/farm"
	"github.com/XOMAK2245/steam-card-xmk/internal/phantom"
	"github.com/XOMAK2245/steam-card-xmk/internal/web"
)

var version = "dev"

func alive(u string) bool {
	c := http.Client{Timeout: 2 * time.Second}
	r, err := c.Get(u + "api/state")
	if err != nil {
		return false
	}
	r.Body.Close()
	return r.StatusCode == 200
}

func main() {
	// Hidden worker mode (Windows): this executable acts as the phantom process.
	if len(os.Args) >= 3 && os.Args[1] == "--phantom-worker" {
		os.Exit(phantom.RunWorker(os.Args[2]))
	}
	port := flag.Int("port", 47890, "local UI port (falls back to a random one if busy)")
	noBrowser := flag.Bool("no-browser", false, "do not open the browser automatically")
	autostart := flag.Bool("autostart", false, "start farming immediately")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("steam-card-xmk", version)
		return
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		// Probably another instance: just show its window and exit.
		u := fmt.Sprintf("http://127.0.0.1:%d/", *port)
		if alive(u) {
			if !*noBrowser {
				showExisting(u)
				time.Sleep(time.Second)
			}
			return
		}
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			log.Fatal(err)
		}
	}

	cfg := config.Load()
	eng := farm.New(cfg)
	srv := &web.Server{Eng: eng, Cfg: cfg, Quit: make(chan struct{})}
	srv.InitDiscord()
	var quitOnce sync.Once
	quit := func() { quitOnce.Do(func() { close(srv.Quit) }) }
	srv.QuitFn = quit
	url := fmt.Sprintf("http://127.0.0.1:%d/", ln.Addr().(*net.TCPAddr).Port)
	hs := &http.Server{Handler: srv.Handler()}
	go hs.Serve(ln)

	fmt.Println("steam-card-xmk", version)
	fmt.Println("UI:", url, "(close this program with Ctrl+C or the Quit button)")
	if s, _ := cfg.Snapshot(); s.SteamID != "" {
		eng.FetchOnce()
		if *autostart {
			eng.Start()
		}
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() { <-sig; quit() }()

	switch {
	case *noBrowser:
		<-srv.Quit
	case runNative(url, srv.Quit): // blocks while the native window is open (Windows)
		quit() // window closed with the cross: shut everything down
	default:
		openApp(url, quit) // Chromium --app window, or the default browser
		<-srv.Quit
	}

	fmt.Println("Shutting down, stopping phantom games...")
	eng.Shutdown()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	hs.Shutdown(ctx)
}
