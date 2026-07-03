package async

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

// TaskClient wraps asynq.Client with typed enqueue helpers.
// Hides JSON marshaling and default task options from callers.
type TaskClient struct {
	client *asynq.Client
}

// NewTaskClient creates a new TaskClient connected to Redis.
func NewTaskClient(redisAddr, redisPassword string) *TaskClient {
	return &TaskClient{
		client: asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr, Password: redisPassword}),
	}
}

// Close shuts down the underlying asynq client.
func (c *TaskClient) Close() error {
	return c.client.Close()
}

func (c *TaskClient) enqueue(typename string, payload interface{}, opts ...asynq.Option) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("async: marshal %s: %w", typename, err)
	}
	task := asynq.NewTask(typename, b, opts...)
	_, err = c.client.Enqueue(task)
	return err
}

// EnqueueSendTelegram queues a Telegram notification for an email.
func (c *TaskClient) EnqueueSendTelegram(accountID, emailID, accountEmail, subject, senderName, snippet string) error {
	return c.enqueue(TypeSendTelegram, PayloadSendTelegram{
		AccountID:    accountID,
		EmailID:      emailID,
		AccountEmail: accountEmail,
		Subject:      subject,
		SenderName:   senderName,
		Snippet:      snippet,
	},
		asynq.MaxRetry(3),
		asynq.Queue("low"),
		asynq.Timeout(15*time.Second),
	)
}

// EnqueueResolveAvatar queues an avatar resolution task.
func (c *TaskClient) EnqueueResolveAvatar(address, name string) error {
	return c.enqueue(TypeResolveAvatar, PayloadResolveAvatar{Address: address, Name: name},
		asynq.MaxRetry(2),
		asynq.Queue("low"),
		asynq.Timeout(10*time.Second),
	)
}

// EnqueueDispatchWebhook queues a webhook dispatch task.
func (c *TaskClient) EnqueueDispatchWebhook(retryID, url, secret string, payload []byte) error {
	return c.enqueue(TypeDispatchWebhook, PayloadDispatchWebhook{RetryID: retryID, URL: url, Secret: secret, Payload: payload},
		asynq.MaxRetry(5),
		asynq.Queue("default"),
		asynq.Timeout(30*time.Second),
	)
}

// EnqueueGenerateAIDraft queues an AI draft generation task.
func (c *TaskClient) EnqueueGenerateAIDraft(accountID, emailID, prompt string) error {
	return c.enqueue(TypeGenerateAIDraft, PayloadGenerateAIDraft{AccountID: accountID, EmailID: emailID, Prompt: prompt},
		asynq.MaxRetry(3),
		asynq.Queue("default"),
		asynq.Timeout(120*time.Second),
	)
}

// EnqueueSendEmail queues an email send task for immediate execution.
func (c *TaskClient) EnqueueSendEmail(jobID, accountID, emailID string) error {
	return c.enqueue(TypeSendEmail, PayloadSendEmail{JobID: jobID, AccountID: accountID, EmailID: emailID},
		asynq.MaxRetry(10),
		asynq.Queue("critical"),
		asynq.Timeout(60*time.Second),
	)
}

// EnqueueSendEmailDelayed queues an email send task for execution after delay.
func (c *TaskClient) EnqueueSendEmailDelayed(jobID, accountID, emailID string, delay time.Duration) error {
	return c.enqueue(TypeSendEmail, PayloadSendEmail{JobID: jobID, AccountID: accountID, EmailID: emailID},
		asynq.MaxRetry(10),
		asynq.Queue("critical"),
		asynq.Timeout(60*time.Second),
		asynq.ProcessIn(delay),
	)
}
