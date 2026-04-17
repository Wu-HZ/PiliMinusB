package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"byted.org/data-speech/asr-tob-demo/sauc/client"
)

type HTTPServer struct {
	wsURL           string
	realtimeWSURL   string
	segmentDuration int
	nonstream       bool
	timeout         time.Duration
	realtimeTimeout time.Duration
}

type TranscribeResponse struct {
	Filename   string `json:"filename,omitempty"`
	Text       string `json:"text"`
	SRT        string `json:"srt,omitempty"`
	Utterances any    `json:"utterances,omitempty"`
	AudioInfo  any    `json:"audio_info,omitempty"`
	Responses  int    `json:"responses"`
	ElapsedMS  int64  `json:"elapsed_ms"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func New(wsURL string, realtimeWSURL string, segmentDuration int, nonstream bool, timeout time.Duration, realtimeTimeout time.Duration) http.Handler {
	s := &HTTPServer{
		wsURL:           wsURL,
		realtimeWSURL:   realtimeWSURL,
		segmentDuration: segmentDuration,
		nonstream:       nonstream,
		timeout:         timeout,
		realtimeTimeout: realtimeTimeout,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/transcribe", s.handleTranscribe)
	mux.HandleFunc("/realtime/ws", s.handleRealtimeWS)
	return mux
}

func (s *HTTPServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":      "sauc-go",
		"status":    "ok",
		"endpoints": []string{"GET /healthz", "POST /transcribe", "GET /realtime/ws"},
	})
}

func (s *HTTPServer) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *HTTPServer) handleTranscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "only POST is supported"})
		return
	}

	filePath, fileName, cleanup, err := persistUpload(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	defer cleanup()

	startedAt := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()

	asrClient := client.NewAsrWsClient(s.wsURL, s.segmentDuration).WithNonstream(s.nonstream)
	result, err := asrClient.TranscribeFile(ctx, filePath)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: err.Error()})
		return
	}

	if strings.EqualFold(r.URL.Query().Get("format"), "srt") {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if result.SRT != "" {
			_, _ = w.Write([]byte(result.SRT))
			return
		}
		_, _ = w.Write([]byte(result.Text))
		return
	}

	writeJSON(w, http.StatusOK, TranscribeResponse{
		Filename:   fileName,
		Text:       result.Text,
		SRT:        result.SRT,
		Utterances: result.Utterances,
		AudioInfo:  result.AudioInfo,
		Responses:  len(result.Responses),
		ElapsedMS:  time.Since(startedAt).Milliseconds(),
	})
}

func persistUpload(r *http.Request) (string, string, func(), error) {
	if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(128 << 20); err != nil {
			return "", "", func() {}, fmt.Errorf("parse multipart form err: %w", err)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			return "", "", func() {}, fmt.Errorf("missing multipart field 'file': %w", err)
		}
		defer file.Close()
		return saveTempFile(file, header.Filename)
	}

	if r.Body == nil {
		return "", "", func() {}, fmt.Errorf("request body is empty")
	}
	filename := r.URL.Query().Get("filename")
	if filename == "" {
		filename = r.Header.Get("X-Filename")
	}
	if filename == "" {
		filename = "audio.wav"
	}
	return saveTempFile(r.Body, filename)
}

func saveTempFile(reader io.Reader, filename string) (string, string, func(), error) {
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".wav"
	}
	tmpFile, err := os.CreateTemp("", "sauc-upload-*"+ext)
	if err != nil {
		return "", "", func() {}, fmt.Errorf("create temp file err: %w", err)
	}

	cleanup := func() {
		_ = os.Remove(tmpFile.Name())
	}

	if _, err := io.Copy(tmpFile, reader); err != nil {
		tmpFile.Close()
		cleanup()
		return "", "", func() {}, fmt.Errorf("save temp file err: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		cleanup()
		return "", "", func() {}, fmt.Errorf("close temp file err: %w", err)
	}
	return tmpFile.Name(), filename, cleanup, nil
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(payload)
}
