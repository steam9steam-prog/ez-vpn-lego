package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"strings"
)

func main() {
	directory := flag.String("dir", ".", "release asset directory")
	address := flag.String("listen", "127.0.0.1:18080", "listen address")
	flag.Parse()
	files := http.FileServer(http.Dir(*directory))
	handler := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case strings.HasSuffix(request.URL.Path, "/getMe"):
			write(response, map[string]any{"ok": true, "result": map[string]any{"id": 123456, "is_bot": true, "first_name": "EZ VPN Lego", "username": "ezvpn_e2e_bot"}})
		case strings.HasSuffix(request.URL.Path, "/getUpdates"):
			write(response, map[string]any{"ok": true, "result": []any{}})
		default:
			files.ServeHTTP(response, request)
		}
	})
	log.Fatal(http.ListenAndServe(*address, handler))
}

func write(response http.ResponseWriter, value any) {
	response.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(response).Encode(value)
}
