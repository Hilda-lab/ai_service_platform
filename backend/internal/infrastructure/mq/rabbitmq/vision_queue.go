package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	amqp "github.com/rabbitmq/amqp091-go"
)

type VisionTaskMessage struct {
	TaskID uint `json:"task_id"`
}

func PublishVisionTask(ctx context.Context, rabbitURL, queue string, taskID uint) error {
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	if _, err := ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return err
	}

	payload, err := json.Marshal(VisionTaskMessage{TaskID: taskID})
	if err != nil {
		return err
	}

	return ch.PublishWithContext(ctx, "", queue, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        payload,
	})
}

func StartVisionConsumer(ctx context.Context, rabbitURL, queue string, process func(taskID uint) error) error {
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return err
	}

	if _, err := ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}

	messages, err := ch.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}

	go func() {
		defer ch.Close()
		defer conn.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-messages:
				if !ok {
					return
				}

				var body VisionTaskMessage
				if err := json.Unmarshal(msg.Body, &body); err != nil {
					_ = msg.Nack(false, false)
					continue
				}
				if body.TaskID == 0 {
					_ = msg.Nack(false, false)
					continue
				}
				if err := process(body.TaskID); err != nil {
					_ = msg.Nack(false, true)
					continue
				}
				_ = msg.Ack(false)
			}
		}
	}()

	return nil
}

func ParseTaskID(raw string) (uint, error) {
	id, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid task id: %w", err)
	}
	return uint(id), nil
}
