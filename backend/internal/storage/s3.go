package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"favorite-things/backend/internal/config"
)

const presignTTL = time.Hour

// Store wraps an S3 or MinIO bucket. Images are served through short-lived presigned GET URLs,
// so buckets can stay private.
type Store struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
	region  string
}

func New(cfg config.Config) *Store {
	creds := credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, "")

	publicEndpoint := cfg.S3PublicEndpoint
	if publicEndpoint == "" {
		publicEndpoint = cfg.S3Endpoint
	}

	return &Store{
		client:  newClient(cfg.S3Endpoint, cfg, creds),
		presign: s3.NewPresignClient(newClient(publicEndpoint, cfg, creds)),
		bucket:  cfg.S3Bucket,
		region:  cfg.S3Region,
	}
}

func newClient(endpoint string, cfg config.Config, creds aws.CredentialsProvider) *s3.Client {
	opts := s3.Options{
		Region:       cfg.S3Region,
		Credentials:  creds,
		UsePathStyle: cfg.S3PathStyle,
	}
	if endpoint != "" {
		opts.BaseEndpoint = aws.String(endpoint)
	}
	return s3.New(opts)
}

// isForbidden reports whether the error is an HTTP 403 from the storage service.
func isForbidden(err error) bool {
	var statusErr interface{ HTTPStatusCode() int }
	return errors.As(err, &statusErr) && statusErr.HTTPStatusCode() == http.StatusForbidden
}

// createBucketInput sets the location constraint for every region except
// us-east-1, where AWS rejects an explicit constraint.
func (s *Store) createBucketInput() *s3.CreateBucketInput {
	input := &s3.CreateBucketInput{Bucket: aws.String(s.bucket)}
	if s.region != "" && s.region != "us-east-1" {
		input.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(s.region),
		}
	}
	return input
}

// EnsureBucket creates the bucket if it does not exist. It retries while the endpoint is still starting up.
func (s *Store) EnsureBucket(ctx context.Context) error {
	var lastErr error
	for attempt := 0; attempt < 30; attempt++ {
		if _, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)}); err == nil {
			return nil
		} else if isForbidden(err) {
			// The bucket may exist but the credentials cannot see it. Creating it
			// would also be denied, so fail with the real cause instead of retrying.
			return fmt.Errorf("bucket %q is not accessible with these credentials (check the access key and s3:ListBucket): %w", s.bucket, err)
		}
		_, err := s.client.CreateBucket(ctx, s.createBucketInput())
		var owned *types.BucketAlreadyOwnedByYou
		if err == nil || errors.As(err, &owned) {
			return nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return fmt.Errorf("ensure bucket %q: %w", s.bucket, lastErr)
}

func (s *Store) Put(ctx context.Context, key, contentType string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	return err
}

func (s *Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *Store) URL(ctx context.Context, key string) (string, error) {
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(presignTTL))
	if err != nil {
		return "", fmt.Errorf("presign %s: %w", key, err)
	}
	return req.URL, nil
}
