package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Connorig/go-blackbox/framework/push/sse"
)

func main() {
	logFile, _ := os.OpenFile("tmp_sse_debug/server.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	defer logFile.Close()
	log.SetOutput(logFile)

	manager := sse.NewManager()
	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("handler start, count=%d", manager.Count())
		manager.Handler("message")(w, r)
		log.Printf("handler end, count=%d", manager.Count())
	})
	go func() {
		time.Sleep(500 * time.Millisecond)
		log.Printf("broadcast-1, count=%d", manager.Count())
		err := manager.Broadcast("message", map[string]string{"hello": "world"})
		log.Printf("broadcast-1 result: %v", err)
		time.Sleep(2 * time.Second)
		_ = manager.Broadcast("message", map[string]string{"second": "event"})
		log.Printf("broadcast-2 done, count=%d", manager.Count())
	}()
	log.Fatal(http.ListenAndServe("127.0.0.1:18080", nil))
}