package client

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/gorilla/websocket"

	"piliminusb/sauc/request"
	"piliminusb/sauc/response"
)

type RealtimeSession struct {
	audio   request.AudioMeta
	url     string
	connect *websocket.Conn

	nextSeq int
	pending []byte

	finished bool

	writeMu   sync.Mutex
	closeOnce sync.Once
}

func NewRealtimeSession(ctx context.Context, url string, audio request.AudioMeta) (*RealtimeSession, error) {
	header, err := request.NewAuthHeader()
	if err != nil {
		return nil, fmt.Errorf("build auth header err: %w", err)
	}

	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, url, header)
	if err != nil {
		return nil, fmt.Errorf("dial realtime websocket err: %w", err)
	}
	log.Printf("realtime logid: %s", resp.Header.Get("X-Tt-Logid"))

	session := &RealtimeSession{
		audio:   audio,
		url:     url,
		connect: conn,
		nextSeq: 2,
	}
	if err := session.sendFullClientRequest(); err != nil {
		session.Close()
		return nil, err
	}
	return session, nil
}

func (s *RealtimeSession) sendFullClientRequest() error {
	fullClientRequest := request.NewFullClientRequest(s.audio, false)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if err := s.connect.WriteMessage(websocket.BinaryMessage, fullClientRequest); err != nil {
		return fmt.Errorf("full client message write websocket err: %w", err)
	}
	_, resp, err := s.connect.ReadMessage()
	if err != nil {
		return fmt.Errorf("full client message read err: %w", err)
	}
	respStruct := response.ParseResponse(resp)
	if respStruct.Code != 0 {
		errMsg := "realtime full request rejected"
		if respStruct.PayloadMsg != nil && respStruct.PayloadMsg.Error != "" {
			errMsg = respStruct.PayloadMsg.Error
		}
		return fmt.Errorf("%s: %d", errMsg, respStruct.Code)
	}
	return nil
}

func (s *RealtimeSession) PushAudio(chunk []byte) error {
	if len(chunk) == 0 {
		return nil
	}
	if s.finished {
		return fmt.Errorf("realtime session already finished")
	}

	if len(s.pending) == 0 {
		s.pending = cloneChunk(chunk)
		return nil
	}

	if err := s.writeAudio(s.pending, false); err != nil {
		return err
	}
	s.pending = cloneChunk(chunk)
	return nil
}

func (s *RealtimeSession) Finish() error {
	if s.finished {
		return nil
	}
	s.finished = true
	if len(s.pending) == 0 {
		return nil
	}
	err := s.writeAudio(s.pending, true)
	s.pending = nil
	return err
}

func (s *RealtimeSession) ReadResponse() (*response.AsrResponse, error) {
	_, message, err := s.connect.ReadMessage()
	if err != nil {
		return nil, err
	}
	return response.ParseResponse(message), nil
}

func (s *RealtimeSession) Close() error {
	var err error
	s.closeOnce.Do(func() {
		if s.connect != nil {
			err = s.connect.Close()
		}
	})
	return err
}

func (s *RealtimeSession) writeAudio(chunk []byte, last bool) error {
	if len(chunk) == 0 {
		return nil
	}

	seq := s.nextSeq
	if last {
		seq = -seq
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	message := request.NewAudioOnlyRequest(seq, chunk)
	if err := s.connect.WriteMessage(websocket.BinaryMessage, message); err != nil {
		return fmt.Errorf("write realtime audio err: %w", err)
	}
	log.Printf("realtime send message: seq=%d bytes=%d", seq, len(chunk))
	s.nextSeq++
	return nil
}

func cloneChunk(chunk []byte) []byte {
	buf := make([]byte, len(chunk))
	copy(buf, chunk)
	return buf
}
