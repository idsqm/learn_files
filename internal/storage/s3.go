package storage

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

type Config struct {
	// Endpoint is used for server-side SDK calls (HeadObject, DeleteObject) made by this
	// process. In Docker it's typically an internal hostname (e.g. http://minio:9000).
	Endpoint string
	// PublicEndpoint is the host baked into presigned URLs handed to clients (browsers,
	// mobile apps). It must be reachable by whoever performs the actual PUT/GET, since the
	// SigV4 signature binds the Host header — a presigned URL signed for "minio:9000" will
	// fail with SignatureDoesNotMatch if accessed via "localhost:9000". Defaults to Endpoint
	// when unset, which is correct for AWS S3 / R2 where there's only one address anyway.
	PublicEndpoint string
	Region         string
	Bucket         string
	AccessKey      string
	SecretKey      string
	UsePathStyle   bool
}

type s3Storage struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

func New(cfg Config) Storage {
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	publicEndpoint := cfg.PublicEndpoint
	if publicEndpoint == "" {
		publicEndpoint = cfg.Endpoint
	}

	awsCfg := aws.Config{
		Region:      region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.UsePathStyle
	})

	presignClient := client
	if publicEndpoint != cfg.Endpoint {
		presignClient = s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			if publicEndpoint != "" {
				o.BaseEndpoint = aws.String(publicEndpoint)
			}
			o.UsePathStyle = cfg.UsePathStyle
		})
	}

	return &s3Storage{
		client:  client,
		presign: s3.NewPresignClient(presignClient),
		bucket:  cfg.Bucket,
	}
}

func (s *s3Storage) GeneratePutURL(ctx context.Context, key, contentType string, ttl time.Duration) (string, time.Time, error) {
	req, err := s.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", time.Time{}, err
	}
	return req.URL, time.Now().Add(ttl), nil
}

func (s *s3Storage) GenerateGetURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (s *s3Storage) HeadObject(ctx context.Context, key string) (*ObjectInfo, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var respErr *smithyhttp.ResponseError
		if errors.As(err, &respErr) && respErr.HTTPStatusCode() == http.StatusNotFound {
			return nil, ErrObjectNotFound
		}
		return nil, err
	}

	return &ObjectInfo{
		SizeBytes:   aws.ToInt64(out.ContentLength),
		ContentType: aws.ToString(out.ContentType),
	}, nil
}

func (s *s3Storage) DeleteObject(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}
