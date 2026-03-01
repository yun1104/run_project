package mq

import (
	"context"
	"encoding/json"
	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer *kafka.Writer
}

type Consumer struct {
	reader *kafka.Reader
}

func NewProducer(brokers []string, topic string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(brokers...),
			Topic:                  topic,
			Balancer:               &kafka.LeastBytes{},
			RequiredAcks:           kafka.RequireOne,
			Async:                  false,
			Compression:            kafka.Snappy,
			BatchSize:              100,
			BatchTimeout:           10,
		},
	}
}

func (p *Producer) Send(ctx context.Context, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	
	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: data,
	})
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

func NewConsumer(brokers []string, topic, groupID string) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  brokers,
			Topic:    topic,
			GroupID:  groupID,
			MinBytes: 10e3,
			MaxBytes: 10e6,
		}),
	}
}

func (c *Consumer) Read(ctx context.Context) ([]byte, error) {
	msg, err := c.reader.ReadMessage(ctx)
	if err != nil {
		return nil, err
	}
	return msg.Value, nil
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
