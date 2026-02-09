package infra

import (
    "time"

    "github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type KafkaProducer struct {
    Producer *kafka.Producer
    Topic    string
}

func NewKafkaProducer(bootstrapServers string, topic string) (*KafkaProducer, error) {
    p, err := kafka.NewProducer(&kafka.ConfigMap{
        "bootstrap.servers": bootstrapServers,
        "client.id":         "job-scheduler-producer",
        "acks":              "all",
    })
    if err != nil {
        return nil, err
    }

    return &KafkaProducer{Producer: p, Topic: topic}, nil
}

func (k *KafkaProducer) Publish(key string, payload []byte) error {
    deliveryChan := make(chan kafka.Event)
    
    err := k.Producer.Produce(&kafka.Message{
        TopicPartition: kafka.TopicPartition{Topic: &k.Topic, Partition: kafka.PartitionAny},
        Key:            []byte(key),
        Value:          payload,
    }, deliveryChan)
    
    if err != nil {
        return err
    }

    e := <-deliveryChan
    m := e.(*kafka.Message)

    if m.TopicPartition.Error != nil {
        return m.TopicPartition.Error
    }
    
    close(deliveryChan)
    return nil
}


func (k *KafkaProducer) Close() {
    k.Producer.Flush(15 * 1000)
    k.Producer.Close()
}

type KafkaConsumer struct {
    Consumer *kafka.Consumer
    Topic    string
}

func NewKafkaConsumer(bootstrapServers string, groupID string, topic string) (*KafkaConsumer, error) {
    c, err := kafka.NewConsumer(&kafka.ConfigMap{
        "bootstrap.servers":  bootstrapServers,
        "group.id":           groupID,
        "auto.offset.reset":  "earliest",
        "enable.auto.commit": false,
    })
    if err != nil {
        return nil, err
    }

    err = c.SubscribeTopics([]string{topic}, nil)
    if err != nil {
        return nil, err
    }

    return &KafkaConsumer{Consumer: c, Topic: topic}, nil
}

func (k *KafkaConsumer) ReadMessage(timeout time.Duration) (*kafka.Message, error) {
    return k.Consumer.ReadMessage(timeout)
}

func (k *KafkaConsumer) CommitMessage(msg *kafka.Message) ([]kafka.TopicPartition, error) {
    return k.Consumer.CommitMessage(msg)
}

func (k *KafkaConsumer) Close() error {
    return k.Consumer.Close()
}
