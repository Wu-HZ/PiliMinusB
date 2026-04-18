package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"piliminusb/config"
	"piliminusb/sauc/request"
)

// Service hosts the sauc HTTP handlers. It is constructed once and its methods
// are registered onto the existing gin router in server/main.go.
type Service struct {
	wsURL                 string
	realtimeWSURL         string
	segmentDuration       int
	nonstream             bool
	timeout               time.Duration
	realtimeTimeout       time.Duration
	transcribeConcurrency int
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

func New(cfg config.SaucConfig) *Service {
	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 2 * time.Hour
	}
	realtimeTimeout := time.Duration(cfg.RealtimeTimeoutSec) * time.Second
	if realtimeTimeout <= 0 {
		realtimeTimeout = 30 * time.Minute
	}
	segmentDuration := cfg.SegmentDuration
	if segmentDuration <= 0 {
		segmentDuration = 200
	}
	wsURL := cfg.WSURL
	if wsURL == "" {
		wsURL = "wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_nostream"
	}
	realtimeWSURL := cfg.RealtimeWSURL
	if realtimeWSURL == "" {
		realtimeWSURL = "wss://openspeech.bytedance.com/api/v3/sauc/bigmodel"
	}
	concurrency := cfg.TranscribeConcurrency
	if concurrency <= 0 {
		concurrency = 3
	}
	return &Service{
		wsURL:                 wsURL,
		realtimeWSURL:         realtimeWSURL,
		segmentDuration:       segmentDuration,
		nonstream:             request.InferNonstream(wsURL),
		timeout:               timeout,
		realtimeTimeout:       realtimeTimeout,
		transcribeConcurrency: concurrency,
	}
}

func (s *Service) Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Service) Transcribe(w http.ResponseWriter, r *http.Request) {
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
	chunkDurationMS := parseChunkDurationMS(r)
	earlyScheduleMS := parseEarlyScheduleMS(r, chunkDurationMS)
	if stat, statErr := os.Stat(filePath); statErr == nil {
		log.Printf(
			"received transcribe upload: filename=%s temp=%s size=%d progressive=%t early_chunks_ms=%v chunk_ms=%d concurrency=%d",
			fileName,
			filePath,
			stat.Size(),
			isProgressiveTranscribe(r),
			earlyScheduleMS,
			chunkDurationMS,
			s.transcribeConcurrency,
		)
	}

	startedAt := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()

	if isProgressiveTranscribe(r) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "streaming is not supported by current server"})
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")

		result, err := s.transcribeByChunks(ctx, filePath, earlyScheduleMS, chunkDurationMS, func(event transcribeProgressEvent) error {
			event.Filename = fileName
			return writeNDJSON(w, flusher, event)
		})
		if err != nil {
			_ = writeNDJSON(w, flusher, transcribeProgressEvent{
				Type:      "error",
				Filename:  fileName,
				Error:     err.Error(),
				ElapsedMS: time.Since(startedAt).Milliseconds(),
			})
			return
		}
		_ = writeNDJSON(w, flusher, transcribeProgressEvent{
			Type:      "done",
			Filename:  fileName,
			Responses: len(result.Responses),
			ElapsedMS: time.Since(startedAt).Milliseconds(),
			Text:      result.Text,
		})
		return
	}

	result, err := s.transcribeByChunks(ctx, filePath, earlyScheduleMS, chunkDurationMS, nil)
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
