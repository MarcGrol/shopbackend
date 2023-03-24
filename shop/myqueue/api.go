package myqueue

import (
	"context"
)

type Task struct {
	UID            string
	WebhookURLPath string
	Payload        []byte
	IsLastAttempt  bool
}

var New func(c context.Context) (TaskQueuer, func(), error)

type TaskQueuer interface {
	Enqueue(c context.Context, task Task) error
	IsLastAttempt(c context.Context, taskUID string) (int32, int32)
}
