package common

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const DefaultSampleRate = 16000

type AudioData struct {
	Content []byte
	PCM     []byte
	Format  string
	Codec   string
	Rate    int
	Bits    int
	Channel int
}

type ProtocolVersion byte
type MessageType byte
type MessageTypeSpecificFlags byte
type SerializationType byte
type CompressionType byte

const (
	PROTOCOL_VERSION = ProtocolVersion(0b0001)

	// Message Type:
	CLIENT_FULL_REQUEST       = MessageType(0b0001)
	CLIENT_AUDIO_ONLY_REQUEST = MessageType(0b0010)
	SERVER_FULL_RESPONSE      = MessageType(0b1001)
	SERVER_ERROR_RESPONSE     = MessageType(0b1111)

	// Message Type Specific Flags
	NO_SEQUENCE       = MessageTypeSpecificFlags(0b0000) // no check sequence
	POS_SEQUENCE      = MessageTypeSpecificFlags(0b0001)
	NEG_SEQUENCE      = MessageTypeSpecificFlags(0b0010)
	NEG_WITH_SEQUENCE = MessageTypeSpecificFlags(0b0011)

	// Message Serialization
	NO_SERIALIZATION = SerializationType(0b0000)
	JSON             = SerializationType(0b0001)

	// Message Compression
	GZIP = CompressionType(0b0001)
)

func GzipCompress(input []byte) []byte {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	w.Write(input)
	w.Close()
	return b.Bytes()
}

func GzipDecompress(input []byte) []byte {
	b := bytes.NewBuffer(input)
	r, _ := gzip.NewReader(b)
	out, _ := ioutil.ReadAll(r)
	r.Close()
	return out
}

// JudgeWav 用于判断字节数组是否为有效的 WAV 文件
func JudgeWav(data []byte) bool {
	if len(data) < 44 {
		return false
	}
	if string(data[0:4]) == "RIFF" && string(data[8:12]) == "WAVE" {
		return true
	}
	return false
}

func ConvertPCMWithPath(audioPath string, sampleRate int) ([]byte, error) {
	args := []string{
		"-v", "error",
		"-nostdin",
		"-y",
		"-probesize", "50M",
		"-analyzeduration", "100M",
	}
	switch strings.ToLower(filepath.Ext(audioPath)) {
	case ".m4s", ".m4a", ".mp4", ".mov":
		args = append(args, "-f", "mp4")
	}
	args = append(
		args,
		"-i", audioPath,
		"-vn",
		"-sn",
		"-dn",
		"-map", "0:a:0?",
		"-acodec", "pcm_s16le",
		"-ac", "1",
		"-ar", strconv.Itoa(sampleRate),
		"-f", "s16le",
		"-",
	)
	cmd := exec.Command("ffmpeg", args...)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("command start error: %v", err)
	}

	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-time.After(10 * time.Minute):
		if err := cmd.Process.Kill(); err != nil {
			fmt.Printf("failed to kill process: %v\n", err)
		}
		<-done
		return nil, fmt.Errorf("process killed as timeout reached")
	case err := <-done:
		if err != nil {
			return nil, fmt.Errorf("process run error: %v (%s)", err, truncateFFmpegError(stderr.Bytes()))
		}
	}

	pcm := out.Bytes()
	if len(pcm) == 0 {
		return nil, fmt.Errorf("ffmpeg produced empty pcm output (%s)", truncateFFmpegError(stderr.Bytes()))
	}
	if stat, statErr := os.Stat(audioPath); statErr == nil && stat.Size() > 1<<20 && len(pcm) < 320 {
		return nil, fmt.Errorf(
			"ffmpeg produced suspiciously short pcm output: input_bytes=%d pcm_bytes=%d (%s)",
			stat.Size(),
			len(pcm),
			truncateFFmpegError(stderr.Bytes()),
		)
	}
	return pcm, nil
}

func LoadAudio(audioPath string, sampleRate int) (*AudioData, error) {
	content, err := ioutil.ReadFile(audioPath)
	if err != nil {
		return nil, fmt.Errorf("read audio file err: %w", err)
	}
	return LoadAudioBytes(content, audioPath, sampleRate)
}

func LoadAudioBytes(content []byte, sourcePath string, sampleRate int) (*AudioData, error) {
	if JudgeWav(content) {
		channelNum, sampWidth, frameRate, _, waveBytes, err := ReadWavInfo(content)
		if err != nil {
			return nil, fmt.Errorf("read wav info err: %w", err)
		}
		if channelNum == 1 && sampWidth == 2 && frameRate == sampleRate {
			return &AudioData{
				Content: content,
				PCM:     waveBytes,
				Format:  "wav",
				Codec:   "raw",
				Rate:    frameRate,
				Bits:    sampWidth * 8,
				Channel: channelNum,
			}, nil
		}
	}

	if sourcePath == "" {
		return nil, fmt.Errorf("audio needs ffmpeg normalization but source path is empty")
	}

	pcm, err := ConvertPCMWithPath(sourcePath, sampleRate)
	if err != nil {
		return nil, fmt.Errorf("convert audio to pcm err: %w", err)
	}
	content, err = BuildWavFromPCM(pcm, sampleRate, 16, 1)
	if err != nil {
		return nil, fmt.Errorf("build wav from pcm err: %w", err)
	}
	return &AudioData{
		Content: content,
		PCM:     pcm,
		Format:  "wav",
		Codec:   "raw",
		Rate:    sampleRate,
		Bits:    16,
		Channel: 1,
	}, nil
}

type WavHeader struct {
	ChunkID       [4]byte
	ChunkSize     uint32
	Format        [4]byte
	Subchunk1ID   [4]byte
	Subchunk1Size uint32
	AudioFormat   uint16
	NumChannels   uint16
	SampleRate    uint32
	ByteRate      uint32
	BlockAlign    uint16
	BitsPerSample uint16
	Subchunk2ID   [4]byte
	Subchunk2Size uint32
}

func ReadWavInfo(data []byte) (int, int, int, int, []byte, error) {
	reader := bytes.NewReader(data)
	var riff [4]byte
	if err := binary.Read(reader, binary.LittleEndian, &riff); err != nil {
		return 0, 0, 0, 0, nil, fmt.Errorf("failed to read WAV riff header: %v", err)
	}
	if string(riff[:]) != "RIFF" {
		return 0, 0, 0, 0, nil, fmt.Errorf("invalid WAV chunk id: %q", string(riff[:]))
	}
	if _, err := reader.Seek(4, io.SeekCurrent); err != nil {
		return 0, 0, 0, 0, nil, fmt.Errorf("failed to skip WAV size: %v", err)
	}
	var wave [4]byte
	if err := binary.Read(reader, binary.LittleEndian, &wave); err != nil {
		return 0, 0, 0, 0, nil, fmt.Errorf("failed to read WAV format: %v", err)
	}
	if string(wave[:]) != "WAVE" {
		return 0, 0, 0, 0, nil, fmt.Errorf("invalid WAV format: %q", string(wave[:]))
	}

	var nchannels int
	var sampwidth int
	var framerate int
	var waveBytes []byte
	for reader.Len() >= 8 {
		var chunkID [4]byte
		if err := binary.Read(reader, binary.LittleEndian, &chunkID); err != nil {
			return 0, 0, 0, 0, nil, fmt.Errorf("failed to read WAV chunk id: %v", err)
		}
		var chunkSize uint32
		if err := binary.Read(reader, binary.LittleEndian, &chunkSize); err != nil {
			return 0, 0, 0, 0, nil, fmt.Errorf("failed to read WAV chunk size: %v", err)
		}
		if int(chunkSize) > reader.Len() {
			return 0, 0, 0, 0, nil, fmt.Errorf("invalid WAV chunk size: %d", chunkSize)
		}
		chunkData := make([]byte, chunkSize)
		if _, err := io.ReadFull(reader, chunkData); err != nil {
			return 0, 0, 0, 0, nil, fmt.Errorf("failed to read WAV chunk %q: %v", string(chunkID[:]), err)
		}
		if chunkSize%2 == 1 {
			if _, err := reader.Seek(1, io.SeekCurrent); err != nil {
				return 0, 0, 0, 0, nil, fmt.Errorf("failed to skip WAV padding: %v", err)
			}
		}

		switch string(chunkID[:]) {
		case "fmt ":
			if len(chunkData) < 16 {
				return 0, 0, 0, 0, nil, fmt.Errorf("invalid fmt chunk size: %d", len(chunkData))
			}
			nchannels = int(binary.LittleEndian.Uint16(chunkData[2:4]))
			framerate = int(binary.LittleEndian.Uint32(chunkData[4:8]))
			sampwidth = int(binary.LittleEndian.Uint16(chunkData[14:16]) / 8)
		case "data":
			waveBytes = chunkData
		}
	}
	if nchannels == 0 || sampwidth == 0 || framerate == 0 {
		return 0, 0, 0, 0, nil, fmt.Errorf("wav fmt chunk not found")
	}
	if len(waveBytes) == 0 {
		return 0, 0, 0, 0, nil, fmt.Errorf("wav data chunk not found")
	}
	nframes := len(waveBytes) / (nchannels * sampwidth)
	return nchannels, sampwidth, framerate, nframes, waveBytes, nil
}

func truncateFFmpegError(stderr []byte) string {
	text := strings.TrimSpace(string(stderr))
	if text == "" {
		return "ffmpeg stderr empty"
	}
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\n", " | ")
	if len(text) > 300 {
		return text[:300] + "..."
	}
	return text
}

func BuildWavFromPCM(pcm []byte, sampleRate int, bits int, channels int) ([]byte, error) {
	if bits%8 != 0 {
		return nil, fmt.Errorf("bits per sample must be divisible by 8")
	}
	if sampleRate <= 0 || channels <= 0 {
		return nil, fmt.Errorf("invalid wav params")
	}

	byteRate := sampleRate * channels * (bits / 8)
	blockAlign := channels * (bits / 8)
	header := WavHeader{
		ChunkID:       [4]byte{'R', 'I', 'F', 'F'},
		ChunkSize:     uint32(36 + len(pcm)),
		Format:        [4]byte{'W', 'A', 'V', 'E'},
		Subchunk1ID:   [4]byte{'f', 'm', 't', ' '},
		Subchunk1Size: 16,
		AudioFormat:   1,
		NumChannels:   uint16(channels),
		SampleRate:    uint32(sampleRate),
		ByteRate:      uint32(byteRate),
		BlockAlign:    uint16(blockAlign),
		BitsPerSample: uint16(bits),
		Subchunk2ID:   [4]byte{'d', 'a', 't', 'a'},
		Subchunk2Size: uint32(len(pcm)),
	}

	var buffer bytes.Buffer
	if err := binary.Write(&buffer, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("write wav header err: %w", err)
	}
	if _, err := buffer.Write(pcm); err != nil {
		return nil, fmt.Errorf("write wav payload err: %w", err)
	}
	return buffer.Bytes(), nil
}
