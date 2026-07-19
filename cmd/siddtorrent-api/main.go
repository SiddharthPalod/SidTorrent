package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/SiddharthPalod/SidTorrent/internal/api"
)

func main() {
	var addr string
	var staticDir string
	flag.StringVar(&addr, "addr", "127.0.0.1:8080", "address for the web UI and API")
	flag.StringVar(&staticDir, "static", "frontend/dist", "directory containing frontend assets")
	flag.Parse()

	fmt.Printf("SiddTorrent web console listening on http://%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, api.NewServer(staticDir)))
}
