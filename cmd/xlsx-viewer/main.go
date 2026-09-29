package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/qtaghdi/xlsx-viewer/internal/httpapi"
	"github.com/qtaghdi/xlsx-viewer/internal/workbook"
)

func main() {
	file := flag.String("file", "", "path to the XLSX workbook to open")
	addr := flag.String("addr", "127.0.0.1:8765", "HTTP listen address")
	browserToken := flag.String("browser-token", "", "browser session token; generated when omitted")
	mcpToken := flag.String("mcp-token", "", "MCP bearer token; generated when omitted")
	flag.Parse()
	if *file == "" {
		fmt.Fprintln(os.Stderr, "usage: xlsx-viewer -file <workbook.xlsx> [-addr 127.0.0.1:8765]")
		os.Exit(2)
	}
	if *browserToken == "" {
		*browserToken = randomToken()
	}
	if *mcpToken == "" {
		*mcpToken = randomToken()
	}
	session, err := workbook.Open(*file)
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()

	server := &http.Server{
		Addr:              *addr,
		Handler:           httpapi.NewWithTokens(session, *browserToken, *mcpToken).Handler(),
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
