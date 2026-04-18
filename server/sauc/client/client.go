package client

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"piliminusb/sauc/common"
	"piliminusb/sauc/request"
	"piliminusb/sauc/response"
)

type Transcript struct {
	Text       string                  `json:"text"`
	SRT        string                  `json:"srt,omitempty"`
	Utterances []response.Utterance    `json:"utterances,omitempty"`
	AudioInfo  response.AudioInfo      `json:"audio_info"`
	Responses  []*response.AsrResponse `json:"responses,omitempty"`
}

type AsrWsClient struct {
	seq             int
	segmentDuration int
	url             string
	nonstream       bool
	connect         *websocket.Conn
}

func NewAsrWsClient(url string, segmentDuration int) *AsrWsClient {
	return &AsrWsClient{
		seq:             1,
		url:             url,
		segmentDuration: segmentDuration,
		nonstream:       request.InferNonstream(url),
	}
}

func (c *AsrWsClient) WithNonstream(nonstream bool) *AsrWsClient {
	c.nonstream = nonstream
	return c
}

func (c *AsrWsClient) readAudioData(filePath string) (*common.AudioData, error) {
	audio, err := common.LoadAudio(filePath, common.DefaultSampleRate)
	if err != nil {
		return nil, fmt.Errorf("load audio err: %w", err)
	}
	return audio, nil
}

func (c *AsrWsClient) getSegmentSize(audio *common.AudioData) int {
	sizePerSec := audio.Channel * (audio.Bits / 8) * audio.Rate
	return sizePerSec * c.segmentDuration / 1000
}

func (c *AsrWsClient) createConnection(ctx context.Context) error {
	header, err := request.NewAuthHeader()
	if err != nil {
		return fmt.Errorf("build auth header err: %w", err)
	}
	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, c.url, header)
	if err != nil {
		return fmt.Errorf("dial websocket err: %w", err)
	}
	log.Printf("logid: %s", resp.Header.Get("X-Tt-Logid"))
	c.connect = conn
	return nil
}

func (c *AsrWsClient) sendFullClientRequest(audio *common.AudioData) error {
	fullClientRequest := request.NewFullClientRequest(request.AudioMeta{
		Format:  audio.Format,
		Codec:   audio.Codec,
		Rate:    audio.Rate,
		Bits:    audio.Bits,
		Channel: audio.Channel,
	}, c.nonstream)
	c.seq++
	err := c.connect.WriteMessage(websocket.BinaryMessage, fullClientRequest)
	if err != nil {
		return fmt.Errorf("full client message write websocket err: %w", err)
	}
	_, resp, err := c.connect.ReadMessage()
	if err != nil {
		return fmt.Errorf("full client message read err: %w", err)
	}
	respStruct := response.ParseResponse(resp)
	log.Println(respStruct)
	return nil
}

func (c *AsrWsClient) sendMessages(segmentSize int, pcm []byte, stopChan <-chan struct{}) error {
	audioSegments := splitAudio(pcm, segmentSize)
	log.Printf(
		"send audio stream: bytes=%d segment_size=%d segments=%d nonstream=%v",
		len(pcm),
		segmentSize,
		len(audioSegments),
		c.nonstream,
	)
	var ticker *time.Ticker
	if !c.nonstream {
		ticker = time.NewTicker(time.Duration(c.segmentDuration) * time.Millisecond)
		defer ticker.Stop()
	}

	for _, segment := range audioSegments {
		select {
		case <-stopChan:
			return nil
		default:
		}

		if ticker != nil {
			select {
			case <-ticker.C:
			case <-stopChan:
				return nil
			}
		}

		seq := c.seq
		if c.seq == len(audioSegments)+1 {
			seq = -c.seq
		}
		message := request.NewAudioOnlyRequest(seq, segment)
		if err := c.connect.WriteMessage(websocket.BinaryMessage, message); err != nil {
			return fmt.Errorf("write message err: %w", err)
		}
		log.Printf("send message: seq: %d", seq)
		c.seq++
	}
	return nil
}

func safeCloseStopChan(stopChan chan<- struct{}) {
	defer func() {
		_ = recover()
	}()
	close(stopChan)
}

func (c *AsrWsClient) recvMessages(resChan chan<- *response.AsrResponse, stopChan chan<- struct{}) {
	defer close(resChan)
	for {
		_, message, err := c.connect.ReadMessage()
		if err != nil {
			return
		}
		resp := response.ParseResponse(message)
		resChan <- resp
		if resp.IsLastPackage {
			return
		}
		if resp.Code != 0 {
			safeCloseStopChan(stopChan)
			return
		}
	}
}

func (c *AsrWsClient) startAudioStream(ctx context.Context, segmentSize int, audio *common.AudioData, resChan chan<- *response.AsrResponse) error {
	stopChan := make(chan struct{})
	go func() {
		<-ctx.Done()
		safeCloseStopChan(stopChan)
		if c.connect != nil {
			_ = c.connect.Close()
		}
	}()
	sendErrChan := make(chan error, 1)
	go func() {
		sendErrChan <- c.sendMessages(segmentSize, audio.Content, stopChan)
	}()
	c.recvMessages(resChan, stopChan)
	if err := <-sendErrChan; err != nil {
		return fmt.Errorf("failed to send audio stream: %w", err)
	}
	return nil
}

func (c *AsrWsClient) Execute(ctx context.Context, filePath string, resChan chan<- *response.AsrResponse) error {
	if filePath == "" {
		close(resChan)
		return errors.New("file path is empty")
	}
	audio, err := c.readAudioData(filePath)
	if err != nil {
		close(resChan)
		return fmt.Errorf("read audio data err: %w", err)
	}
	return c.ExecuteAudio(ctx, audio, resChan)
}

func (c *AsrWsClient) ExecuteAudio(ctx context.Context, audio *common.AudioData, resChan chan<- *response.AsrResponse) error {
	c.seq = 1
	if c.url == "" {
		close(resChan)
		return errors.New("url is empty")
	}
	if audio == nil {
		close(resChan)
		return errors.New("audio is empty")
	}
	segmentSize := c.getSegmentSize(audio)

	err := c.createConnection(ctx)
	if err != nil {
		close(resChan)
		return fmt.Errorf("create connection err: %w", err)
	}
	defer c.connect.Close()

	if err = c.sendFullClientRequest(audio); err != nil {
		close(resChan)
		return fmt.Errorf("send full request err: %w", err)
	}
	if err = c.startAudioStream(ctx, segmentSize, audio, resChan); err != nil {
		return fmt.Errorf("start audio stream err: %w", err)
	}
	return nil
}

func (c *AsrWsClient) Excute(ctx context.Context, filePath string, resChan chan<- *response.AsrResponse) error {
	return c.Execute(ctx, filePath, resChan)
}

func (c *AsrWsClient) transcribeAudio(ctx context.Context, audio *common.AudioData) (*Transcript, error) {
	resChan := make(chan *response.AsrResponse)
	errChan := make(chan error, 1)
	go func() {
		errChan <- c.ExecuteAudio(ctx, audio, resChan)
	}()

	var responses []*response.AsrResponse
	for res := range resChan {
		responses = append(responses, res)
	}

	if err := <-errChan; err != nil {
		return nil, err
	}

	result := &Transcript{Responses: responses}
	for _, item := range responses {
		if item.Code != 0 {
			errMsg := "asr service returned non-zero code"
			if item.PayloadMsg != nil && item.PayloadMsg.Error != "" {
				errMsg = item.PayloadMsg.Error
			}
			return nil, fmt.Errorf("%s: %d", errMsg, item.Code)
		}
	}

	var latestPayload *response.AsrResponsePayload
	mergedUtterances := make([]response.Utterance, 0)
	for _, item := range responses {
		if item.PayloadMsg == nil {
			continue
		}
		latestPayload = item.PayloadMsg
		mergedUtterances = mergeUtterances(
			mergedUtterances,
			item.PayloadMsg.Result.Utterances,
		)
	}

	if latestPayload != nil {
		result.AudioInfo = latestPayload.AudioInfo
	}
	if len(mergedUtterances) > 0 {
		result.Utterances = mergedUtterances
		result.Text = buildTranscriptText(mergedUtterances)
		result.SRT = response.BuildSRT(mergedUtterances)
	} else if latestPayload != nil {
		result.Text = strings.TrimSpace(latestPayload.Result.Text)
		durationMS := latestPayload.AudioInfo.Duration
		if durationMS <= 0 {
			durationMS = estimateAudioDurationMS(audio)
		}
		if result.Text != "" && durationMS > 0 {
			log.Printf(
				"fallback utterance synthesis: duration_ms=%d text_len=%d",
				durationMS,
				len(result.Text),
			)
			result.Utterances = []response.Utterance{
				{
					Definite:  true,
					StartTime: 0,
					EndTime:   durationMS,
					Text:      result.Text,
				},
			}
			result.SRT = response.BuildSRT(result.Utterances)
		}
	}

	return result, nil
}

func (c *AsrWsClient) TranscribeAudio(ctx context.Context, audio *common.AudioData) (*Transcript, error) {
	if audio == nil {
		return nil, errors.New("audio is empty")
	}
	return c.transcribeAudio(ctx, audio)
}

func (c *AsrWsClient) TranscribeFile(ctx context.Context, filePath string) (*Transcript, error) {
	if filePath == "" {
		return nil, errors.New("file path is empty")
	}
	audio, err := c.readAudioData(filePath)
	if err != nil {
		return nil, fmt.Errorf("read audio data err: %w", err)
	}
	return c.transcribeAudio(ctx, audio)
}

func mergeUtterances(existing []response.Utterance, incoming []response.Utterance) []response.Utterance {
	if len(incoming) == 0 {
		return existing
	}

	indexMap := make(map[int]int, len(existing))
	for index, utterance := range existing {
		indexMap[utterance.StartTime] = index
	}

	for _, utterance := range incoming {
		if strings.TrimSpace(utterance.Text) == "" || utterance.EndTime <= utterance.StartTime {
			continue
		}
		if index, ok := indexMap[utterance.StartTime]; ok {
			existing[index] = utterance
			continue
		}
		indexMap[utterance.StartTime] = len(existing)
		existing = append(existing, utterance)
	}

	sort.Slice(existing, func(i int, j int) bool {
		if existing[i].StartTime == existing[j].StartTime {
			return existing[i].EndTime < existing[j].EndTime
		}
		return existing[i].StartTime < existing[j].StartTime
	})
	return existing
}

func buildTranscriptText(utterances []response.Utterance) string {
	if len(utterances) == 0 {
		return ""
	}

	lines := make([]string, 0, len(utterances))
	for _, utterance := range utterances {
		text := strings.TrimSpace(utterance.Text)
		if text != "" {
			lines = append(lines, text)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func estimateAudioDurationMS(audio *common.AudioData) int {
	if audio == nil || audio.Rate <= 0 || audio.Channel <= 0 || audio.Bits <= 0 {
		return 0
	}
	bytesPerSecond := audio.Rate * audio.Channel * (audio.Bits / 8)
	if bytesPerSecond <= 0 {
		return 0
	}
	if len(audio.PCM) > 0 {
		return len(audio.PCM) * 1000 / bytesPerSecond
	}
	return len(audio.Content) * 1000 / bytesPerSecond
}

func splitAudio(data []byte, segmentSize int) [][]byte {
	if segmentSize <= 0 {
		return nil
	}
	var segments [][]byte
	for i := 0; i < len(data); i += segmentSize {
		end := i + segmentSize
		if end > len(data) {
			end = len(data)
		}
		segments = append(segments, data[i:end])
	}
	return segments
}
