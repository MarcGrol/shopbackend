package myqueue

import (
	"context"
	"os"
)

type fakeTaskQueue struct {
}

func init() {
	if os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
		New = newFakeQueue
	}
}

func newFakeQueue(c context.Context) (TaskQueuer, func(), error) {
	return &fakeTaskQueue{}, func() {
	}, nil
}

func (q *fakeTaskQueue) Enqueue(c context.Context, task Task) error {
	return nil
}

func (q *fakeTaskQueue) IsLastAttempt(c context.Context, taskUID string) (int32, int32) {
	return 0, 0
}
