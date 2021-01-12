package main

import (
	"context"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yobert/alsa"
)

func audio(ctx context.Context, audioChannelStore *sync.Map) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
		if err != nil {
			panic(err)
		}
		c := make(chan []byte)
		remoteAddr := conn.RemoteAddr().String()
		audioChannelStore.Store(remoteAddr, c)
		defer audioChannelStore.Delete(remoteAddr)
		log.Println("audio open", remoteAddr)
		for data := range c {
			err = conn.WriteMessage(websocket.BinaryMessage, data)
			if err != nil {
				log.Println(err)
				break
			}
		}
	}
}

// audio over wave
func audioWave(ctx context.Context, audioChannelStore *sync.Map) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := make(chan []byte)
		remoteAddr := r.RemoteAddr
		audioChannelStore.Store(remoteAddr, c)
		defer audioChannelStore.Delete(remoteAddr)
		w.Header().Set("Content-Type", "audio/wav")
		n, err := NewWaveHeader(math.MaxUint32, 2, 48000, 16).Write(w)
		if err != nil {
			log.Println(err)
			return
		}
		for data := range c {
			_, err = w.Write(data[n:])
			if err != nil {
				log.Println(err)
				return
			}
		}
	}
}

func audioWorker(ctx context.Context, audioChannelStore *sync.Map) {
	recordDev := getAudioRecordDev()
	log.Println("audio device", recordDev)
	for {
		err := func() error {
			err := recordDev.Open()
			if err != nil {
				panic(err)
			}
			defer recordDev.Close()
			recordDev.NegotiateChannels(2)
			recordDev.NegotiateFormat(alsa.S16_LE)
			recordDev.NegotiateRate(48000)
			n, _ := recordDev.NegotiateBufferSize(4096, 8192, 12000, 24000)
			if err = recordDev.Prepare(); err != nil {
				panic(err)
			}
			log.Println("Audio buffer size", n)
			bodySize := recordDev.BytesPerFrame() * n
			headerBytes := NewWaveHeader(uint32(bodySize), 2, 48000, 16).Bytes()
			for {
				select {
				case <-ctx.Done():
					log.Println("Audio Stopping...")
					return nil
				default:
					clientCount := 0
					audioChannelStore.Range(func(_, _ interface{}) bool {
						clientCount++
						return true
					})
					if clientCount == 0 {
						time.Sleep(time.Second)
						recordDev.Close()
						continue
					}
					buff := make([]byte, bodySize+44)
					copy(buff, headerBytes)
					err = recordDev.Read(buff[len(headerBytes):])
					if err != nil {
						return err
					}
					audioChannelStore.Range(func(_, v interface{}) bool {
						c := v.(chan []byte)
						select {
						case c <- buff:
						default:
							log.Println("loss audio")
						}
						return true
					})
				}
			}
		}()
		if err == nil {
			break
		}
		log.Println(err)
	}
}

func getAudioRecordDev() *alsa.Device {
	cards, err := alsa.OpenCards()
	if err != nil {
		panic(err)
	}
	defer alsa.CloseCards(cards)
	for i := range cards {
		devs, err := cards[i].Devices()
		if err != nil {
			panic(err)
		}
		for j := range devs {
			if devs[j].Type == alsa.PCM && devs[j].Record {
				return devs[j]
			}
		}
	}
	return nil
}
