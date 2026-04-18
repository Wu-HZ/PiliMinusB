package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"piliminusb/sauc/client"
	"piliminusb/sauc/common"
	"piliminusb/sauc/response"
)

const defaultTranscribeChunkDurationMS = 5 * 60 * 1000
const defaultFirstTranscribeChunkDurationMS = 2 * 60 * 1000

type transcribeProgressEvent struct {
	Type        string               `json:"type"`
	Filename    string               `json:"filename,omitempty"`
	ChunkIndex  int                  `json:"chunk_index,omitempty"`
	TotalChunks int                  `json:"total_chunks,omitempty"`
	Utterances  []response.Utterance `json:"utterances,omitempty"`
	Text        string               `json:"text,omitempty"`
	Responses   int                  `json:"responses,omitempty"`
	ElapsedMS   int64                `json:"elapsed_ms,omitempty"`
	Error       string               `json:"error,omitempty"`
}

type audioChunk struct {
	OffsetMS int
	Audio    *common.AudioData
}

func parseChunkDurationMS(r *http.Request) int {
	if value := strings.TrimSpace(r.URL.Query().Get("chunk_ms")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			return parsed
		}
	}
	return defaultTranscribeChunkDurationMS
}

// parseEarlyScheduleMS returns the durations (in ms) for the leading chunks so
// that early subtitles can arrive faster than a single long first chunk allows.
// Preferred input: `early_chunks_ms=120000,120000` (CSV). Falls back to
// `first_chunk_ms` when only that legacy param is present. Returns an empty
// slice when neither is provided — the caller then uses chunkDurationMS for
// every chunk (up to the default first-chunk cap).
func parseEarlyScheduleMS(r *http.Request, chunkDurationMS int) []int {
	raw := strings.TrimSpace(r.URL.Query().Get("early_chunks_ms"))
	if raw != "" {
		parts := strings.Split(raw, ",")
		schedule := make([]int, 0, len(parts))
		for _, p := range parts {
			if v, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && v > 0 {
				schedule = append(schedule, v)
			}
		}
		if len(schedule) > 0 {
			return schedule
		}
	}
	if legacy := strings.TrimSpace(r.URL.Query().Get("first_chunk_ms")); legacy != "" {
		if v, err := strconv.Atoi(legacy); err == nil && v > 0 {
			return []int{v}
		}
	}
	if chunkDurationMS > 0 {
		return []int{minInt(defaultFirstTranscribeChunkDurationMS, chunkDurationMS)}
	}
	return []int{defaultFirstTranscribeChunkDurationMS}
}

func isProgressiveTranscribe(r *http.Request) bool {
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("progressive"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// transcribeByChunks splits the audio file according to earlyScheduleMS +
// chunkDurationMS and sends each chunk to Volc concurrently (bounded by
// s.transcribeConcurrency). Chunk results are emitted to onChunk in original
// order so the client sees subtitles stitch together from the top of the
// video.
func (s *Service) transcribeByChunks(
	ctx context.Context,
	filePath string,
	earlyScheduleMS []int,
	chunkDurationMS int,
	onChunk func(event transcribeProgressEvent) error,
) (*client.Transcript, error) {
	audio, err := common.LoadAudio(filePath, common.DefaultSampleRate)
	if err != nil {
		return nil, fmt.Errorf("load audio err: %w", err)
	}
	log.Printf(
		"loaded audio: content_bytes=%d pcm_bytes=%d rate=%d bits=%d channels=%d",
		len(audio.Content),
		len(audio.PCM),
		audio.Rate,
		audio.Bits,
		audio.Channel,
	)

	chunks, err := splitAudioChunks(audio, earlyScheduleMS, chunkDurationMS)
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("audio is empty after splitting")
	}

	if onChunk != nil {
		if err := onChunk(transcribeProgressEvent{
			Type:        "ready",
			TotalChunks: len(chunks),
		}); err != nil {
			return nil, err
		}
	}

	result := &client.Transcript{
		Utterances: make([]response.Utterance, 0),
		Responses:  make([]*response.AsrResponse, 0),
	}
	bytesPerSecond := audio.Rate * audio.Channel * (audio.Bits / 8)
	if bytesPerSecond > 0 {
		result.AudioInfo.Duration = len(audio.PCM) * 1000 / bytesPerSecond
	}

	concurrency := s.transcribeConcurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(chunks) {
		concurrency = len(chunks)
	}

	// Per-chunk slots hold completed results. Emission is strictly ordered:
	// when chunk i finishes we flush everything in results[nextEmit...] that
	// has landed so far. This keeps NDJSON ordering aligned with video time
	// even though chunks complete out of order.
	type chunkOutput struct {
		transcript *client.Transcript
		offsetMS   int
	}
	outputs := make([]*chunkOutput, len(chunks))

	var emitMu sync.Mutex
	textParts := make([]string, 0, len(chunks))
	startedAt := time.Now()
	nextEmit := 0
	tryEmit := func() error {
		emitMu.Lock()
		defer emitMu.Unlock()
		for nextEmit < len(outputs) && outputs[nextEmit] != nil {
			out := outputs[nextEmit]
			shifted := shiftUtterances(out.transcript.Utterances, out.offsetMS)
			result.Utterances = append(result.Utterances, shifted...)
			result.Responses = append(result.Responses, out.transcript.Responses...)
			if trimmed := strings.TrimSpace(out.transcript.Text); trimmed != "" {
				textParts = append(textParts, trimmed)
			}
			if onChunk != nil {
				if err := onChunk(transcribeProgressEvent{
					Type:        "chunk",
					ChunkIndex:  nextEmit + 1,
					TotalChunks: len(chunks),
					Utterances:  shifted,
					Text:        out.transcript.Text,
					Responses:   len(result.Responses),
					ElapsedMS:   time.Since(startedAt).Milliseconds(),
				}); err != nil {
					return err
				}
			}
			nextEmit++
		}
		return nil
	}

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(concurrency)

	for i := range chunks {
		i := i
		chunk := chunks[i]
		eg.Go(func() error {
			if err := egCtx.Err(); err != nil {
				return err
			}
			asrClient := client.NewAsrWsClient(
				s.wsURL,
				s.segmentDuration,
			).WithNonstream(s.nonstream)
			log.Printf(
				"transcribe chunk %d/%d: offset_ms=%d wav_bytes=%d pcm_bytes=%d",
				i+1,
				len(chunks),
				chunk.OffsetMS,
				len(chunk.Audio.Content),
				len(chunk.Audio.PCM),
			)
			chunkResult, err := asrClient.TranscribeAudio(egCtx, chunk.Audio)
			if err != nil {
				return err
			}
			log.Printf(
				"chunk result %d/%d: utterances=%d text_len=%d responses=%d",
				i+1,
				len(chunks),
				len(chunkResult.Utterances),
				len(strings.TrimSpace(chunkResult.Text)),
				len(chunkResult.Responses),
			)

			emitMu.Lock()
			outputs[i] = &chunkOutput{transcript: chunkResult, offsetMS: chunk.OffsetMS}
			emitMu.Unlock()

			return tryEmit()
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	result.Text = strings.Join(textParts, "\n")
	result.SRT = response.BuildSRT(result.Utterances)
	return result, nil
}

// splitAudioChunks carves `audio` into a slice of chunks. The first
// `len(earlyScheduleMS)` chunks take their duration from the schedule; every
// remaining chunk uses chunkDurationMS. An empty schedule falls back to a
// single default-sized first chunk plus chunkDurationMS for the rest.
func splitAudioChunks(audio *common.AudioData, earlyScheduleMS []int, chunkDurationMS int) ([]audioChunk, error) {
	if audio == nil {
		return nil, fmt.Errorf("audio is empty")
	}
	if chunkDurationMS <= 0 {
		chunkDurationMS = defaultTranscribeChunkDurationMS
	}
	if len(earlyScheduleMS) == 0 {
		earlyScheduleMS = []int{minInt(defaultFirstTranscribeChunkDurationMS, chunkDurationMS)}
	}

	frameSize := audio.Channel * (audio.Bits / 8)
	if frameSize <= 0 {
		return nil, fmt.Errorf("invalid audio frame size")
	}

	bytesPerSecond := audio.Rate * frameSize
	if bytesPerSecond <= 0 {
		return nil, fmt.Errorf("invalid audio byte rate")
	}

	regularChunkSize := bytesPerSecond * chunkDurationMS / 1000
	regularChunkSize -= regularChunkSize % frameSize
	if regularChunkSize <= 0 {
		regularChunkSize = len(audio.PCM)
	}

	chunkSizeFor := func(i int) int {
		if i < len(earlyScheduleMS) {
			size := bytesPerSecond * earlyScheduleMS[i] / 1000
			size -= size % frameSize
			if size > 0 {
				return size
			}
		}
		return regularChunkSize
	}

	chunks := make([]audioChunk, 0, len(audio.PCM)/regularChunkSize+1+len(earlyScheduleMS))
	index := 0
	for start := 0; start < len(audio.PCM); index++ {
		currentChunkSize := chunkSizeFor(index)
		end := start + currentChunkSize
		if end > len(audio.PCM) {
			end = len(audio.PCM)
		}
		chunkPCM := make([]byte, end-start)
		copy(chunkPCM, audio.PCM[start:end])
		chunkWav, err := common.BuildWavFromPCM(
			chunkPCM,
			audio.Rate,
			audio.Bits,
			audio.Channel,
		)
		if err != nil {
			return nil, err
		}
		offsetMS := start * 1000 / bytesPerSecond
		chunks = append(chunks, audioChunk{
			OffsetMS: offsetMS,
			Audio: &common.AudioData{
				Content: chunkWav,
				PCM:     chunkPCM,
				Format:  "wav",
				Codec:   "raw",
				Rate:    audio.Rate,
				Bits:    audio.Bits,
				Channel: audio.Channel,
			},
		})
		start = end
	}
	return chunks, nil
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func shiftUtterances(utterances []response.Utterance, offsetMS int) []response.Utterance {
	if offsetMS == 0 || len(utterances) == 0 {
		return utterances
	}

	shifted := make([]response.Utterance, len(utterances))
	for i, utterance := range utterances {
		current := utterance
		current.StartTime += offsetMS
		current.EndTime += offsetMS
		if len(utterance.Words) != 0 {
			current.Words = make([]response.Word, len(utterance.Words))
			for j, word := range utterance.Words {
				current.Words[j] = word
				current.Words[j].StartTime += offsetMS
				current.Words[j].EndTime += offsetMS
			}
		}
		shifted[i] = current
	}
	return shifted
}

func writeNDJSON(w http.ResponseWriter, flusher http.Flusher, payload any) error {
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(payload); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
