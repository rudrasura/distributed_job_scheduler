package infra

import (
    "context"
    "fmt"
    "log"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Client struct {
    Client *s3.Client
}

func NewS3Client(endpoint string) (*S3Client, error) {
    cfg, err := config.LoadDefaultConfig(context.TODO(),
        config.WithRegion("us-east-1"),
        config.WithCredentialsProvider(aws.AnonymousCredentials{}),
    )
    if err != nil {
        return nil, fmt.Errorf("unable to load SDK config, %v", err)
    }

    client := s3.NewFromConfig(cfg, func(o *s3.Options) {
        o.BaseEndpoint = aws.String(endpoint)
        o.UsePathStyle = true // Required for LocalStack
    })

    return &S3Client{Client: client}, nil
}

func (s *S3Client) EnsureBucket(bucketName string) error {
    _, err := s.Client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
        Bucket: aws.String(bucketName),
    })
    
    if err != nil {
        log.Printf("Bucket %s not found, creating...", bucketName)
        _, err = s.Client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
            Bucket: aws.String(bucketName),
        })
        if err != nil {
            return fmt.Errorf("failed to create bucket: %v", err)
        }
    }
    return nil
}
