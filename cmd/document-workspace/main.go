package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/qtaghdi/document-workspace/internal/formats/xlsx"
	"github.com/qtaghdi/document-workspace/internal/transport/httpapi"
)

func main() {
	file := flag.String("file", "", "path to the XLSX workbook to open")
	transport := flag.String("transport", "http", "MCP transport: http or stdio")
	addr := flag.String("addr", "127.0.0.1:8765", "HTTP listen address")
	stdioBrowser := flag.Bool("stdio-browser", true, "serve a loopback browser fallback in stdio mode")
	stdioBrowserAddr := flag.String("stdio-browser-addr", "127.0.0.1:0", "loopback address for the stdio browser fallback")
	browserToken := flag.String("browser-token", "", "browser session token; generated when omitted")
	mcpToken := flag.String("mcp-token", "", "MCP bearer token; generated when omitted")
	flag.Parse()
	if *file == "" {
		fmt.Fprintln(os.Stderr, "usage: document-workspace -file <workbook.xlsx> [-transport http|stdio] [-addr 127.0.0.1:8765]")
		os.Exit(2)
	}
	if *transport != "http" && *transport != "stdio" {
		fmt.Fprintf(os.Stderr, "unsupported transport %q: use http or stdio\n", *transport)
		os.Exit(2)
	}
	if *browserToken == "" {
		*browserToken = randomToken()
	}
	if *mcpToken == "" {
		*mcpToken = randomToken()
	}
	session, err := xlsx.Open(*file)
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()

	api := httpapi.NewWithTokens(session, *browserToken, *mcpToken)
	if *transport == "stdio" {
		var err error
		if *stdioBrowser {
			err = api.RunStdioWithBrowser(context.Background(), *stdioBrowserAddr)
		} else {
			err = api.RunStdio(context.Background())
		}
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	server := &http.Server{
		Addr:              *addr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("viewer: http://%s/?token=%s", *addr, *browserToken)
	log.Printf("mcp: http://%s/mcp (Authorization: Bearer %s)", *addr, *mcpToken)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func randomToken() string {
	var value [24]byte
	if _, err := rand.Read(value[:]); err != nil {
		log.Fatal(err)
	}
	return hex.EncodeToString(value[:])
}
