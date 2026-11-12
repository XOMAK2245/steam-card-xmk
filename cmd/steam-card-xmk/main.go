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
	"syscall"
	"time"

	"github.com/XOMAK2245/steam-card-xmk/internal/config"
	"github.com/XOMAK2245/steam-card-xmk/internal/farm"
	"github.com/XOMAK2245/steam-card-xmk/internal/web"
)

var version = "dev"

func main() {
	port := flag.Int("port", 47890, "local UI port (falls back to a random one if busy)")
	noBrowser := flag.Bool("no-browser", false, "do not open the browser automatically")
	autostart := flag.Bool("autostart", false, "start farming immediately")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("steam-card-xmk", version)
		return
	}

	cfg := config.Load()
	eng := farm.New(cfg)
	srv := &web.Server{Eng: eng, Cfg: cfg, Quit: make(chan struct{})}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			log.Fatal(err)
		}
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/", ln.Addr().(*net.TCPAddr).Port)
	hs := &http.Server{Handler: srv.Handler()}
	go hs.Serve(ln)

	fmt.Println("steam-card-xmk", version)
	fmt.Println("UI:", url, "(close this program with Ctrl+C or the Quit button)")
	if !*noBrowser {
		openBrowser(url)
	}
	if s, _ := cfg.Snapshot(); s.SteamID != "" {
		eng.FetchOnce()
		if *autostart {
			eng.Start()
		}
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sig:
	case <-srv.Quit:
	}
	fmt.Println("Shutting down, stopping phantom games...")
	eng.Shutdown()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	hs.Shutdown(ctx)
}
