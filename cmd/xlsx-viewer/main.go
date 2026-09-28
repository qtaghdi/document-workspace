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
	token := flag.String("token", "", "session token; generated when omitted")
	flag.Parse()
	if *file == "" {
		fmt.Fprintln(os.Stderr, "usage: xlsx-viewer -file <workbook.xlsx> [-addr 127.0.0.1:8765]")
		os.Exit(2)
	}
	if *token == "" {
		*token = randomToken()
	}
	session, err := workbook.Open(*file)
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()

	server := &http.Server{
		Addr:              *addr,
		Handler:           httpapi.New(session, *token).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("viewer: http://%s/?token=%s", *addr, *token)
	log.Printf("mcp: http://%s/mcp (Authorization: Bearer %s)", *addr, *token)
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
