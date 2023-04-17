package myqueue

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	grpcCodes "google.golang.org/grpc/codes"
	grpcStatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type gcloudTaskQueue struct {
	client *cloudtasks.Client
}

func init() {
	if os.Getenv("GOOGLE_CLOUD_PROJECT") != "" {
		New = newGcloudQueue
	}
}

func newGcloudQueue(c context.Context) (TaskQueuer, func(), error) {
	cloudTaskClient, err := cloudtasks.NewClient(c)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating cloudtask-client: %s", err)
	}
	return &gcloudTaskQueue{
			client: cloudTaskClient,
		}, func() {
			cloudTaskClient.Close()
		}, nil
}

func (q *gcloudTaskQueue) Enqueue(c context.Context, task Task) error {
	taskUID := composeTaskName(task.UID)
	_, err := q.client.CreateTask(c, &taskspb.CreateTaskRequest{
		Parent: composeQueueName(),
		Task: &taskspb.Task{
			Name:         taskUID,                                          // de-duplicate
			ScheduleTime: timestamppb.New(time.Now().Add(time.Second * 5)), // delay to give shop some time for redirect
			MessageType: &taskspb.Task_AppEngineHttpRequest{
				AppEngineHttpRequest: &taskspb.AppEngineHttpRequest{
					HttpMethod:  taskspb.HttpMethod_PUT,
					RelativeUri: task.WebhookURLPath,
					Body:        task.Payload,
				},
			},
			View: taskspb.Task_FULL,
		},
	})
	if err != nil {
		rsp, ok := grpcStatus.FromError(err)
		if ok && rsp.Code() == grpcCodes.AlreadyExists {
			log.Printf("task with id %s already exists -> ignore\n", taskUID)
			// Convert error into success
			return nil
		}
		return fmt.Errorf("error submitting task to queue: %s", err)
	}
	return nil
}

func composeQueueName() string {
	projectId := os.Getenv("GOOGLE_CLOUD_PROJECT")
	locationId := os.Getenv("LOCATION_ID")
	queueName := os.Getenv("QUEUE_NAME")
	if queueName == "" {
		queueName = "default"
	}
	return fmt.Sprintf("projects/%s/locations/%s/queues/%s", projectId, locationId, queueName)
}

func composeTaskName(taskUID string) string {
	return fmt.Sprintf("%s/tasks/%s", composeQueueName(), taskUID)
}

func (q *gcloudTaskQueue) IsLastAttempt(c context.Context, taskUID string) (int32, int32) {
	var numRetries int32 = 0
	var maxRetries int32 = -1

	queue, err := q.getQueue(c, composeQueueName())
	if err != nil {
		return numRetries, maxRetries
	}

	if queue.RetryConfig != nil {
		maxRetries = queue.RetryConfig.MaxAttempts
	}

	task, err := q.getTask(c, taskUID)
	if err != nil {
		return numRetries, maxRetries
	}

	// Determine if this is the last attempt
	return task.DispatchCount, maxRetries
}

func (q *gcloudTaskQueue) getQueue(c context.Context, queueName string) (*taskspb.Queue, error) {
	// find characteristics of the queue
	queue, err := q.client.GetQueue(c, &taskspb.GetQueueRequest{
		Name: composeQueueName(),
	})
	if err != nil {
		return nil, fmt.Errorf("error getting queue with name %s: %s", queueName, err)
	}
	return queue, nil
}

func (q *gcloudTaskQueue) getTask(c context.Context, taskUID string) (*taskspb.Task, error) {
	// find characteristics of the task
	task, err := q.client.GetTask(c, &taskspb.GetTaskRequest{
		Name: composeTaskName(taskUID),
	})
	if err != nil {
		return nil, fmt.Errorf("error getting task with uid %s: %s", taskUID, err)
	}
	return task, nil
}
