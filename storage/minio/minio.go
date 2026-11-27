// Package minio adapts the official MinIO SDK to storage.Provider (also S3 compatible).
package minio

import (
	"context"
	"errors"
	"github.com/HieronimusMorgan/common-golang/storage"
	sdk "github.com/minio/minio-go/v7"
	"io"
	"time"
)

type Provider struct {
	Client *sdk.Client
	Bucket string
}

func (p Provider) valid() error {
	if p.Client == nil || p.Bucket == "" {
		return errors.New("client and bucket required")
	}
	return nil
}
func (p Provider) Put(ctx context.Context, o storage.Object, r io.Reader) error {
	if err := p.valid(); err != nil {
		return err
	}
	_, err := p.Client.PutObject(ctx, p.Bucket, o.Key, r, o.Size, sdk.PutObjectOptions{ContentType: o.ContentType, UserMetadata: map[string]string{"sha256": o.SHA256}})
	return err
}
func (p Provider) Get(ctx context.Context, k string) (io.ReadCloser, error) {
	if err := p.valid(); err != nil {
		return nil, err
	}
	o, err := p.Client.GetObject(ctx, p.Bucket, k, sdk.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	if _, err = o.Stat(); err != nil {
		_ = o.Close()
		return nil, err
	}
	return o, nil
}
func (p Provider) Delete(ctx context.Context, k string) error {
	if err := p.valid(); err != nil {
		return err
	}
	return p.Client.RemoveObject(ctx, p.Bucket, k, sdk.RemoveObjectOptions{})
}
func (p Provider) PresignGet(ctx context.Context, k string, ttl time.Duration) (string, error) {
	if err := p.valid(); err != nil {
		return "", err
	}
	u, err := p.Client.PresignedGetObject(ctx, p.Bucket, k, ttl, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

var _ storage.Provider = Provider{}
