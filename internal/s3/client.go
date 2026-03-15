package s3

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Client wraps the AWS S3 client for R2-compatible storage.
type Client struct {
	client *s3.Client
	bucket string
}

// NewClient creates a new S3-compatible client.
func NewClient(endpoint, bucket, accessKey, secretKey string) *Client {
	cfg := aws.Config{
		Region: "auto",
		Credentials: credentials.NewStaticCredentialsProvider(
			accessKey, secretKey, "",
		),
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})
	return &Client{client: client, bucket: bucket}
}

// Upload uploads a file to S3/R2 with the given key.
func (c *Client) Upload(ctx context.Context, key string, body io.Reader, contentType string) error {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	}
	_, err := c.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("s3 upload %s: %w", key, err)
	}
	return nil
}

// HeadObject checks if an object exists.
func (c *Client) HeadObject(ctx context.Context, key string) (bool, error) {
	_, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return false, nil
	}
	return true, nil
}
