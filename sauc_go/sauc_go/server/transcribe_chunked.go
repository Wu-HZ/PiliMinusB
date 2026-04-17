package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"byted.org/data-speech/asr-tob-demo/sauc/client"
	"byted.org/data-speech/asr-tob-demo/sauc/common"
	"byted.org/data-speech/asr-tob-demo/sauc/response"
)

const defaultTranscribeChunkDurationMS = 5 * 60 * 1000

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

func isProgressiveTranscribe(r *http.Request) bool {
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("progressive"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (s *HTTPServer) transcribeByChunks(
	ctx context.Context,
	filePath string,
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

	chunks, err := splitAudioChunks(audio, chunkDurationMS)
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

	textParts := make([]string, 0, len(chunks))
	startedAt := time.Now()
	for index, chunk := range chunks {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		asrClient := client.NewAsrWsClient(
			s.wsURL,
			s.segmentDuration,
		).WithNonstream(s.nonstream)
		log.Printf(
			"transcribe chunk %d/%d: offset_ms=%d wav_bytes=%d pcm_bytes=%d",
			index+1,
			len(chunks),
			chunk.OffsetMS,
			len(chunk.Audio.Content),
			len(chunk.Audio.PCM),
		)
		chunkResult, err := asrClient.TranscribeAudio(ctx, chunk.Audio)
		if err != nil {
			return nil, err
		}
		log.Printf(
			"chunk result %d/%d: utterances=%d text_len=%d responses=%d",
			index+1,
			len(chunks),
			len(chunkResult.Utterances),
			len(strings.TrimSpace(chunkResult.Text)),
			len(chunkResult.Responses),
		)

		shiftedUtterances := shiftUtterances(chunkResult.Utterances, chunk.OffsetMS)
		result.Utterances = append(result.Utterances, shiftedUtterances...)
		result.Responses = append(result.Responses, chunkResult.Responses...)
		if strings.TrimSpace(chunkResult.Text) != "" {
			textParts = append(textParts, strings.TrimSpace(chunkResult.Text))
		}

		if onChunk != nil {
			if err := onChunk(transcribeProgressEvent{
				Type:        "chunk",
				ChunkIndex:  index + 1,
				TotalChunks: len(chunks),
				Utterances:  shiftedUtterances,
				Text:        chunkResult.Text,
				Responses:   len(result.Responses),
				ElapsedMS:   time.Since(startedAt).Milliseconds(),
			}); err != nil {
				return nil, err
			}
		}
	}

	result.Text = strings.Join(textParts, "\n")
	result.SRT = response.BuildSRT(result.Utterances)
	return result, nil
}

func splitAudioChunks(audio *common.AudioData, chunkDurationMS int) ([]audioChunk, error) {
	if audio == nil {
		return nil, fmt.Errorf("audio is empty")
	}
	if chunkDurationMS <= 0 {
		chunkDurationMS = defaultTranscribeChunkDurationMS
	}

	frameSize := audio.Channel * (audio.Bits / 8)
	if frameSize <= 0 {
		return nil, fmt.Errorf("invalid audio frame size")
	}

	bytesPerSecond := audio.Rate * frameSize
	chunkSize := bytesPerSecond * chunkDurationMS / 1000
	chunkSize -= chunkSize % frameSize
	if chunkSize <= 0 {
		chunkSize = len(audio.PCM)
	}

	chunks := make([]audioChunk, 0, len(audio.PCM)/chunkSize+1)
	for start := 0; start < len(audio.PCM); start += chunkSize {
		end := start + chunkSize
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
	}
	return chunks, nil
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
