package infra

import (
    "context"
    "fmt"
    "log"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/sqs"
)

type SQSClient struct {
    Client   *sqs.Client
    QueueURL string
}

func NewSQSClient(endpoint, queueName string) (*SQSClient, error) {
    cfg, err := config.LoadDefaultConfig(context.TODO(),
        config.WithRegion("us-east-1"),
        config.WithCredentialsProvider(aws.AnonymousCredentials{}),
    )
    if err != nil {
        return nil, fmt.Errorf("unable to load SDK config, %v", err)
    }

    client := sqs.NewFromConfig(cfg, func(o *sqs.Options) {
        o.BaseEndpoint = aws.String(endpoint)
    })

    // Get Queue URL
    out, err := client.GetQueueUrl(context.TODO(), &sqs.GetQueueUrlInput{
        QueueName: aws.String(queueName),
    })
    if err != nil {
        // Try creating if not exists (for tests/dev)
        log.Printf("Queue %s not found, attempting to create...", queueName)
        createOut, createErr := client.CreateQueue(context.TODO(), &sqs.CreateQueueInput{
            QueueName: aws.String(queueName),
        })
        if createErr != nil {
             return nil, fmt.Errorf("failed to get or create queue url: %v", err)
        }
        return &SQSClient{Client: client, QueueURL: *createOut.QueueUrl}, nil
    }

    return &SQSClient{Client: client, QueueURL: *out.QueueUrl}, nil
}

func (s *SQSClient) SendMessage(ctx context.Context, body string) error {
    _, err := s.Client.SendMessage(ctx, &sqs.SendMessageInput{
        MessageBody: aws.String(body),
        QueueUrl:    &s.QueueURL,
    })
    return err
}

func (s *SQSClient) ReceiveMessages(ctx context.Context, maxMessages int32, waitTime int32) (*sqs.ReceiveMessageOutput, error) {
    return s.Client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
        QueueUrl:            &s.QueueURL,
        MaxNumberOfMessages: maxMessages,
        WaitTimeSeconds:     waitTime,
    })
}

func (s *SQSClient) DeleteMessage(ctx context.Context, receiptHandle string) error {
    _, err := s.Client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
        QueueUrl:      &s.QueueURL,
        ReceiptHandle: &receiptHandle,
    })
    return err
}
