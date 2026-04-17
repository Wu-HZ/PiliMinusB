package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"byted.org/data-speech/asr-tob-demo/sauc/client"
	"byted.org/data-speech/asr-tob-demo/sauc/request"
	"byted.org/data-speech/asr-tob-demo/sauc/server"
)

var mode = flag.String("mode", "file", "run mode: file or http")
var filePath = flag.String("file", "", "audio file path for file mode")
var wsURL = flag.String("url", "wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_nostream", "request url")
var realtimeWSURL = flag.String("realtime_url", "wss://openspeech.bytedance.com/api/v3/sauc/bigmodel", "realtime request url")
var segmentDuration = flag.Int("seg_duration", 200, "audio duration(ms) per packet")
var listenAddr = flag.String("listen", ":8090", "http listen address")
var timeout = flag.Duration("timeout", 2*time.Minute, "request timeout")
var realtimeTimeout = flag.Duration("realtime_timeout", 30*time.Minute, "realtime websocket timeout")
var outputFormat = flag.String("output", "json", "file mode output: json, text or srt")
var nonstreamMode = flag.String("nonstream", "auto", "nonstream mode: auto, true or false")

func main() {
	flag.Parse()
	initLog()

	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "file":
		if err := runFileMode(); err != nil {
			log.Fatalf("file mode failed: %v", err)
		}
	case "http":
		if err := runHTTPMode(); err != nil {
			log.Fatalf("http mode failed: %v", err)
		}
	default:
		log.Fatalf("unsupported mode: %s", *mode)
	}
}

func initLog() {
	file, err := os.OpenFile("run.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("failed to open log file: %v", err)
	}
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)
	log.SetOutput(io.MultiWriter(os.Stdout, file))
}

func runFileMode() error {
	if strings.TrimSpace(*filePath) == "" {
		return fmt.Errorf("file mode requires -file")
	}

	c := client.NewAsrWsClient(*wsURL, *segmentDuration).WithNonstream(resolveNonstream(*wsURL, *nonstreamMode))
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	result, err := c.TranscribeFile(ctx, *filePath)
	if err != nil {
		return err
	}

	switch strings.ToLower(strings.TrimSpace(*outputFormat)) {
	case "text":
		fmt.Println(result.Text)
	case "srt":
		fmt.Println(result.SRT)
	default:
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	return nil
}

func runHTTPMode() error {
	nonstream := resolveNonstream(*wsURL, *nonstreamMode)
	handler := server.New(*wsURL, *realtimeWSURL, *segmentDuration, nonstream, *timeout, *realtimeTimeout)
	log.Printf("http server listening on %s", *listenAddr)
	return http.ListenAndServe(*listenAddr, handler)
}

func resolveNonstream(url string, mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "auto":
		return request.InferNonstream(url)
	case "true":
		return true
	case "false":
		return false
	default:
		log.Fatalf("unsupported -nonstream value: %s", mode)
		return false
	}
}
