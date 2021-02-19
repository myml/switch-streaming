package main

import (
	"context"
	"embed"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"

	_ "net/http/pprof"
)

//go:embed index.html
var indexHTML embed.FS

func main() {
	var videoChannelStore sync.Map
	var audioChannelStore sync.Map

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		videoWorker(ctx, &videoChannelStore)
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		audioWorker(ctx, &audioChannelStore)
		wg.Done()
	}()

	http.Handle("/", http.FileServer(http.FS(indexHTML)))
	http.HandleFunc("/audio", audio(ctx, &audioChannelStore))
	http.HandleFunc("/audio.wave", audioWave(ctx, &audioChannelStore))
	http.HandleFunc("/video", video(ctx, &videoChannelStore))
	http.HandleFunc("/video.mjpeg", videoMJPEG(ctx, &videoChannelStore))
	server := &http.Server{Addr: ":8080"}
	server.RegisterOnShutdown(cancel)

	go onSignal(ctx, func() {
		log.Println("Http Server Stopping...")
		server.Shutdown(context.Background())
	})
	go func() {
		server.ListenAndServe()
		wg.Done()
	}()
	wg.Wait()
}

func onSignal(ctx context.Context, f func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	select {
	case <-ch:
	case <-ctx.Done():
	}
	f()
}
