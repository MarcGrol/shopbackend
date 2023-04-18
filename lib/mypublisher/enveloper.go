package mypublisher

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"github.com/MarcGrol/shopbackend/lib/myevents"
	"github.com/MarcGrol/shopbackend/lib/mytime"
)

type enveloper struct {
	nower mytime.Nower
}

func newEnveloper(nower mytime.Nower) enveloper {
	return enveloper{
		nower: nower,
	}
}

func (e enveloper) do(topic string, event myevents.Event) (myevents.EventEnvelope, error) {
	jsonPayload, err := json.Marshal(event)
	if err != nil {
		return myevents.EventEnvelope{}, fmt.Errorf("error marshalling request-payload: %s", err)
	}
	envelope := myevents.EventEnvelope{
		Topic:         topic,
		AggregateUID:  event.GetAggregateName(),
		EventTypeName: event.GetEventTypeName(),
		EventPayload:  string(jsonPayload),
		Published:     false,
	}

	// In order to be idempotent, we do NOT use an uuid to identify the event
	envelope.UID, err = checksum(envelope)
	if err != nil {
		return myevents.EventEnvelope{}, fmt.Errorf("error checksumming request-payload: %s", err)
	}
	// In order to be idempotent, we exclude timestamp from the checksum
	envelope.CreatedAt = e.nower.Now()

	return envelope, nil
}

func checksum(envlp myevents.EventEnvelope) (string, error) {
	asJson, err := json.Marshal(envlp)
	if err != nil {
		return "", err
	}

	sha2 := sha256.New()
	_, err = io.WriteString(sha2, string(asJson))
	if err != nil {
		return "", err
	}
	checkSum := base64.RawURLEncoding.EncodeToString(sha2.Sum(nil))
	return checkSum, nil
}
