package response

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"

	"piliminusb/sauc/common"
)

type AudioInfo struct {
	Duration int `json:"duration"`
}

type Word struct {
	EndTime   int    `json:"end_time"`
	StartTime int    `json:"start_time"`
	Text      string `json:"text"`
}

type Utterance struct {
	Definite  bool   `json:"definite"`
	EndTime   int    `json:"end_time"`
	StartTime int    `json:"start_time"`
	Text      string `json:"text"`
	Words     []Word `json:"words"`
}

type Result struct {
	Text       string      `json:"text"`
	Utterances []Utterance `json:"utterances,omitempty"`
}

type AsrResponsePayload struct {
	AudioInfo AudioInfo `json:"audio_info"`
	Result    Result    `json:"result"`
	Error     string    `json:"error,omitempty"`
}

type AsrResponse struct {
	Code            int                 `json:"code"`
	Event           int                 `json:"event"`
	IsLastPackage   bool                `json:"is_last_package"`
	PayloadSequence int32               `json:"payload_sequence"`
	PayloadSize     int                 `json:"payload_size"`
	PayloadMsg      *AsrResponsePayload `json:"payload_msg"`
}

func ParseResponse(msg []byte) *AsrResponse {
	var result AsrResponse

	headerSize := msg[0] & 0x0f
	messageType := common.MessageType(msg[1] >> 4)
	messageTypeSpecificFlags := common.MessageTypeSpecificFlags(msg[1] & 0x0f)
	serializationMethod := common.SerializationType(msg[2] >> 4)
	messageCompression := common.CompressionType(msg[2] & 0x0f)
	payload := msg[headerSize*4:]
	// 解析messageTypeSpecificFlags
	if messageTypeSpecificFlags&0x01 != 0 {
		result.PayloadSequence = int32(binary.BigEndian.Uint32(payload[:4]))
		payload = payload[4:]
	}
	if messageTypeSpecificFlags&0x02 != 0 {
		result.IsLastPackage = true
	}
	if messageTypeSpecificFlags&0x04 != 0 {
		result.Event = int(binary.BigEndian.Uint32(payload[:4]))
		payload = payload[4:]
	}

	// 解析messageType
	switch messageType {
	case common.SERVER_FULL_RESPONSE:
		result.PayloadSize = int(binary.BigEndian.Uint32(payload[:4]))
		payload = payload[4:]
	case common.SERVER_ERROR_RESPONSE:
		result.Code = int(binary.BigEndian.Uint32(payload[:4]))
		result.PayloadSize = int(binary.BigEndian.Uint32(payload[4:8]))
		payload = payload[8:]
	}

	if len(payload) == 0 {
		return &result
	}

	// 是否压缩
	if messageCompression == common.GZIP {
		payload = common.GzipDecompress(payload)
	}

	// 解析payload
	var asrResponse AsrResponsePayload
	switch serializationMethod {
	case common.JSON:
		_ = json.Unmarshal(payload, &asrResponse)
	case common.NO_SERIALIZATION:
	}
	result.PayloadMsg = &asrResponse
	return &result
}

func BuildSRT(utterances []Utterance) string {
	if len(utterances) == 0 {
		return ""
	}

	var builder strings.Builder
	for idx, utterance := range utterances {
		builder.WriteString(fmt.Sprintf("%d\n", idx+1))
		builder.WriteString(fmt.Sprintf("%s --> %s\n", formatTimestamp(utterance.StartTime), formatTimestamp(utterance.EndTime)))
		builder.WriteString(strings.TrimSpace(utterance.Text))
		builder.WriteString("\n\n")
	}
	return strings.TrimSpace(builder.String())
}

func formatTimestamp(ms int) string {
	hours := ms / 3600000
	minutes := (ms % 3600000) / 60000
	seconds := (ms % 60000) / 1000
	milliseconds := ms % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, milliseconds)
}
