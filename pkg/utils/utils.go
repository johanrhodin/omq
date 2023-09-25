package utils

import (
	"encoding/binary"
	"time"

	config "github.com/rabbitmq/omq/pkg/config"
)

func WaitBetweenMessages(rate int) {
	if rate > 0 {
		t := time.Duration(1000/float64(rate)) * time.Millisecond
		time.Sleep(t)
	}
}

func MessageBody(cfg config.Config) []byte {
	b := make([]byte, cfg.Size)
	binary.BigEndian.PutUint32(b[0:], uint32(1234)) // currently unused, for compatibility with perf-test
	return b
}

func UpdatePayload(useMillis bool, payload *[]byte) *[]byte {
	if useMillis {
		binary.BigEndian.PutUint64((*payload)[4:], uint64(time.Now().UnixMilli()))
	} else {
		binary.BigEndian.PutUint64((*payload)[4:], uint64(time.Now().UnixNano()))
	}
	return payload
}

func CalculateEndToEndLatency(payload *[]byte) float64 {
	timeSent := binary.BigEndian.Uint64((*payload)[4:])
	now := uint64(time.Now().UnixNano())
	latency := now - timeSent
	return (float64(latency) / 1000000000)
}