package mypublisher

import (
	"encoding/json"
	"fmt"
	"io"
)

type PushRequest struct {
	Message      PushMessage
	Subscription string
}

type PushMessage struct {
	Attributes map[string]string
	Data       []byte
	ID         string `json:"message_id"`
}

func ParseEventEnvelope(r io.Reader) (EventEnvelope, error) {
	msg := PushRequest{}
	err := json.NewDecoder(r).Decode(&msg)
	if err != nil {
		return EventEnvelope{}, fmt.Errorf("error parsing push-request:%s", err)
	}
	envlp := EventEnvelope{}
	err = json.Unmarshal(msg.Message.Data, &envlp)
	if err != nil {
		return EventEnvelope{}, fmt.Errorf("error parsing envelope:%s", err)
	}

	return envlp, nil
}
