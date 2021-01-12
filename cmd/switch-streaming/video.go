package main

import (
	"context"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/korandiz/v4l"
	"github.com/korandiz/v4l/fmt/mjpeg"
)

// video over websocket
func video(ctx context.Context, videoChannelStore *sync.Map) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
		if err != nil {
			panic(err)
		}
		c := make(chan []byte)
		videoChannelStore.Store(r.RemoteAddr, c)
		defer videoChannelStore.Delete(r.RemoteAddr)
		log.Println("video open", r.RemoteAddr)
		for data := range c {
			err = conn.WriteMessage(websocket.BinaryMessage, data)
			if err != nil {
				log.Println(err)
				break
			}
		}
	}
}

// video over mjpeg
func videoMJPEG(ctx context.Context, videoChannelStore *sync.Map) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := make(chan []byte)
		remoteAddr := r.RemoteAddr
		videoChannelStore.Store(remoteAddr, c)
		defer videoChannelStore.Delete(remoteAddr)
		m := multipart.NewWriter(w)
		defer m.Close()
		w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+m.Boundary())
		h := textproto.MIMEHeader{}
		h.Set("Content-Type", "image/jpeg")
		for data := range c {
			h.Set("Content-Length", fmt.Sprint(len(data)))
			mw, err := m.CreatePart(h)
			if err != nil {
				log.Println(err)
				break
			}
			mw.Write(data)
		}
	}
}

func videoWorker(ctx context.Context, videoChannelStore *sync.Map) {
	cam, err := v4l.Open("/dev/video0")
	if err != nil {
		panic(err)
	}
	defer cam.Close()
	cfgs, err := cam.ListConfigs()
	if err != nil {
		panic(err)
	}
	var cfg *v4l.DeviceConfig
	for i := range cfgs {
		if cfgs[i].Format == mjpeg.FourCC && cfgs[i].FPS.N <= 30 {
			cfg = &cfgs[i]
			break
		}
	}
	if cfg == nil {
		panic("Video not find config")
	}
	log.Printf("Video config %+v \n", cfg)
	err = cam.SetConfig(*cfg)
	if err != nil {
		panic(err)
	}
	ticker := time.NewTicker(time.Second / time.Duration(cfg.FPS.N))
	defer ticker.Stop()
	turnoff := true
	for {
		select {
		case <-ctx.Done():
			log.Println("Video Stopping...")
			return
		case <-ticker.C:
			clientCount := 0
			videoChannelStore.Range(func(_, _ interface{}) bool {
				clientCount++
				return true
			})
			if clientCount == 0 {
				if !turnoff {
					turnoff = true
					cam.TurnOff()
				}
				time.Sleep(time.Second)
				continue
			}
			if turnoff {
				turnoff = false
				cam.TurnOn()
			}
			var buf *v4l.Buffer
			for buf == nil || buf.Size() <= 4 {
				buf, err = cam.Capture()
				if err != nil {
					panic(err)
				}
			}
			data := make([]byte, buf.Len())
			buf.Read(data)
			videoChannelStore.Range(func(_, v interface{}) bool {
				c := v.(chan []byte)
				select {
				case c <- data:
				default:
					log.Println("loss video")
				}
				return true
			})
		}
	}
}
