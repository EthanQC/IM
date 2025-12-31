package minio

import (
	"context"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/EthanQC/IM/services/file_service/internal/ports/out"
)

// MinIOStorage MinIO对象存储实现
type MinIOStorage struct {
	client *minio.Client
}

// NewMinIOStorage 创建MinIO存储
func NewMinIOStorage(endpoint, accessKeyID, secretAccessKey string, useSSL bool) (out.ObjectStorage, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}

	return &MinIOStorage{client: client}, nil
}

// GeneratePresignedPutURL 生成预签名上传URL
func (s *MinIOStorage) GeneratePresignedPutURL(ctx context.Context, bucket, objectKey, contentType string, expiry time.Duration) (string, error) {
	presignedURL, err := s.client.PresignedPutObject(ctx, bucket, objectKey, expiry)
	if err != nil {
		return "", err
	}
	return presignedURL.String(), nil
}

// GeneratePresignedGetURL 生成预签名下载URL
func (s *MinIOStorage) GeneratePresignedGetURL(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error) {
	reqParams := make(url.Values)
	presignedURL, err := s.client.PresignedGetObject(ctx, bucket, objectKey, expiry, reqParams)
	if err != nil {
		return "", err
	}
	return presignedURL.String(), nil
}

// Upload 上传文件
func (s *MinIOStorage) Upload(ctx context.Context, bucket, objectKey string, reader io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, bucket, objectKey, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

// Delete 删除文件
func (s *MinIOStorage) Delete(ctx context.Context, bucket, objectKey string) error {
	return s.client.RemoveObject(ctx, bucket, objectKey, minio.RemoveObjectOptions{})
}

// Exists 检查文件是否存在
func (s *MinIOStorage) Exists(ctx context.Context, bucket, objectKey string) (bool, error) {
	_, err := s.client.StatObject(ctx, bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		errResponse := minio.ToErrorResponse(err)
		if errResponse.Code == "NoSuchKey" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetObjectInfo 获取对象信息
func (s *MinIOStorage) GetObjectInfo(ctx context.Context, bucket, objectKey string) (*out.ObjectInfo, error) {
	stat, err := s.client.StatObject(ctx, bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		return nil, err
	}

	return &out.ObjectInfo{
		Key:          stat.Key,
		Size:         stat.Size,
		ContentType:  stat.ContentType,
		LastModified: stat.LastModified,
		ETag:         stat.ETag,
	}, nil
}

// EnsureBucket 确保Bucket存在
func (s *MinIOStorage) EnsureBucket(ctx context.Context, bucket, region string) error {
	exists, err := s.client.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}

	if !exists {
		return s.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: region})
	}

	return nil
}
