package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"byted.org/data-speech/asr-tob-demo/sauc/client"
	"byted.org/data-speech/asr-tob-demo/sauc/request"
	"byted.org/data-speech/asr-tob-demo/sauc/response"
)

var realtimeUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type realtimeClientMessage struct {
	Type string `json:"type"`
}

type realtimeServerMessage struct {
	Type       string                `json:"type"`
	Text       string                `json:"text,omitempty"`
	Error      string                `json:"error,omitempty"`
	Audio      *request.AudioMeta    `json:"audio,omitempty"`
	Response   *response.AsrResponse `json:"response,omitempty"`
	Utterances []response.Utterance  `json:"utterances,omitempty"`
	AudioInfo  *response.AudioInfo   `json:"audio_info,omitempty"`
}

func (s *HTTPServer) handleRealtimeWS(w http.ResponseWriter, r *http.Request) {
	audioMeta, err := parseRealtimeAudioMeta(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	conn, err := realtimeUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade realtime websocket err: %v", err)
		return
	}
	defer conn.Close()

	ctx, cancel := s.newRealtimeContext()
	defer cancel()

	session, err := client.NewRealtimeSession(ctx, s.realtimeWSURL, audioMeta)
	if err != nil {
		_ = writeRealtimeJSON(conn, &sync.Mutex{}, realtimeServerMessage{
			Type:  "error",
			Error: err.Error(),
		})
		return
	}
	defer session.Close()

	go func() {
		<-ctx.Done()
		_ = session.Close()
		_ = conn.Close()
	}()

	var writeMu sync.Mutex
	if err := writeRealtimeJSON(conn, &writeMu, realtimeServerMessage{
		Type:  "ready",
		Audio: &audioMeta,
	}); err != nil {
		return
	}

	upstreamDone := make(chan struct{})
	go func() {
		defer close(upstreamDone)
		for {
			resp, err := session.ReadResponse()
			if err != nil {
				if ctx.Err() == nil {
					_ = writeRealtimeJSON(conn, &writeMu, realtimeServerMessage{
						Type:  "error",
						Error: err.Error(),
					})
				}
				cancel()
				return
			}

			msgType := "partial"
			if resp.IsLastPackage {
				msgType = "final"
			}
			event := realtimeServerMessage{
				Type:     msgType,
				Response: resp,
			}
			if resp.PayloadMsg != nil {
				event.Text = resp.PayloadMsg.Result.Text
				event.Utterances = resp.PayloadMsg.Result.Utterances
				event.AudioInfo = &resp.PayloadMsg.AudioInfo
			}

			if err := writeRealtimeJSON(conn, &writeMu, event); err != nil {
				cancel()
				return
			}

			if resp.Code != 0 || resp.IsLastPackage {
				cancel()
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			<-upstreamDone
			return
		default:
		}

		messageType, message, err := conn.ReadMessage()
		if err != nil {
			cancel()
			<-upstreamDone
			return
		}

		switch messageType {
		case websocket.BinaryMessage:
			if err := session.PushAudio(message); err != nil {
				_ = writeRealtimeJSON(conn, &writeMu, realtimeServerMessage{
					Type:  "error",
					Error: err.Error(),
				})
				cancel()
				<-upstreamDone
				return
			}
		case websocket.TextMessage:
			var req realtimeClientMessage
			if err := json.Unmarshal(message, &req); err != nil {
				_ = writeRealtimeJSON(conn, &writeMu, realtimeServerMessage{
					Type:  "error",
					Error: fmt.Sprintf("invalid control message: %v", err),
				})
				cancel()
				<-upstreamDone
				return
			}
			switch req.Type {
			case "end":
				if err := session.Finish(); err != nil {
					_ = writeRealtimeJSON(conn, &writeMu, realtimeServerMessage{
						Type:  "error",
						Error: err.Error(),
					})
					cancel()
					<-upstreamDone
					return
				}
			case "cancel":
				cancel()
				<-upstreamDone
				return
			case "ping":
				if err := writeRealtimeJSON(conn, &writeMu, realtimeServerMessage{Type: "pong"}); err != nil {
					cancel()
					<-upstreamDone
					return
				}
			default:
				_ = writeRealtimeJSON(conn, &writeMu, realtimeServerMessage{
					Type:  "error",
					Error: fmt.Sprintf("unsupported control type: %s", req.Type),
				})
				cancel()
				<-upstreamDone
				return
			}
		default:
			_ = writeRealtimeJSON(conn, &writeMu, realtimeServerMessage{
				Type:  "error",
				Error: "unsupported websocket message type",
			})
			cancel()
			<-upstreamDone
			return
		}
	}
}

func (s *HTTPServer) newRealtimeContext() (context.Context, context.CancelFunc) {
	if s.realtimeTimeout <= 0 {
		return context.WithCancel(context.Background())
	}
	return context.WithTimeout(context.Background(), s.realtimeTimeout)
}

func parseRealtimeAudioMeta(r *http.Request) (request.AudioMeta, error) {
	meta := request.AudioMeta{
		Format:  queryOrDefault(r, "format", "pcm"),
		Codec:   queryOrDefault(r, "codec", "raw"),
		Rate:    16000,
		Bits:    16,
		Channel: 1,
	}

	var err error
	if meta.Rate, err = queryInt(r, "rate", meta.Rate); err != nil {
		return request.AudioMeta{}, err
	}
	if meta.Bits, err = queryInt(r, "bits", meta.Bits); err != nil {
		return request.AudioMeta{}, err
	}
	if meta.Channel, err = queryInt(r, "channel", meta.Channel); err != nil {
		return request.AudioMeta{}, err
	}
	return meta, nil
}

func queryOrDefault(r *http.Request, key string, fallback string) string {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback
	}
	return value
}

func queryInt(r *http.Request, key string, fallback int) (int, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return parsed, nil
}

func writeRealtimeJSON(conn *websocket.Conn, mu *sync.Mutex, payload realtimeServerMessage) error {
	mu.Lock()
	defer mu.Unlock()
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	defer conn.SetWriteDeadline(time.Time{})
	return conn.WriteJSON(payload)
}
