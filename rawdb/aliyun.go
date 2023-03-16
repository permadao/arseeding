package rawdb

import (
	"bytes"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/permadao/arseeding/schema"
	"io"
)

// refer https://help.aliyun.com/document_detail/32157.html?spm=a2c4g.11186623.0.0.1a4b32bcxaC4kR
const (
	ossErrorNoSuchKey = "NoSuchKey"
)

type AliyunDB struct {
	bucketPrefix string
	client       *oss.Client
}

func NewAliyunDB(endpoint, accKey, accessKeySecret, bktPrefix string) (*AliyunDB, error) {
	client, err := oss.New(endpoint, accKey, accessKeySecret)
	if err != nil {
		return nil, err
	}

	err = createAliyunBucket(client, bktPrefix)
	if err != nil {
		return nil, err
	}

	log.Info("run with aliyun oss success")

	return &AliyunDB{
		bucketPrefix: bktPrefix,
		client:       client,
	}, nil
}

func (a *AliyunDB) Put(bucket, key string, value []byte) (err error) {
	bkt, err := a.client.Bucket(getS3Bucket(a.bucketPrefix, bucket))
	if err != nil {
		return err
	}

	return bkt.PutObject(key, bytes.NewReader(value))
}

func (a *AliyunDB) Get(bucket, key string) (data []byte, err error) {
	bkt, err := a.client.Bucket(getS3Bucket(a.bucketPrefix, bucket))
	if err != nil {
		return
	}

	body, err := bkt.GetObject(key)
	if err != nil {
		// handleOSSErr make file non-existent errors converted to schema.ErrNotFound
		return nil, handleOSSErr(err)
	}

	defer func(body io.ReadCloser) {
		_ = body.Close()
	}(body)

	data, err = io.ReadAll(body)
	return
}

func (a *AliyunDB) GetAllKey(bucket string) (keys []string, err error) {
	bkt, err := a.client.Bucket(getS3Bucket(a.bucketPrefix, bucket))
	if err != nil {
		return
	}

	keys = make([]string, 0)

	startAfter := ""
	continueToken := ""
	var lsRes oss.ListObjectsResultV2

	for {
		lsRes, err = bkt.ListObjectsV2(oss.StartAfter(startAfter), oss.ContinuationToken(continueToken))
		if err != nil {
			break
		}
		for _, object := range lsRes.Objects {
			keys = append(keys, object.Key)
		}
		if lsRes.IsTruncated {
			startAfter = lsRes.StartAfter
			continueToken = lsRes.NextContinuationToken
		} else {
			break
		}
	}

	if len(keys) == 0 {
		err = schema.ErrNotExist
	}

	return
}

func (a *AliyunDB) Delete(bucket, key string) (err error) {
	bkt, err := a.client.Bucket(getS3Bucket(a.bucketPrefix, bucket))
	if err != nil {
		return
	}

	return bkt.DeleteObject(key)
}

func (a *AliyunDB) Close() (err error) {
	return
}

func createAliyunBucket(svc *oss.Client, prefix string) error {
	bucketNames := []string{
		schema.ChunkBucket,
		schema.TxDataEndOffSetBucket,
		schema.TxMetaBucket,
		schema.ConstantsBucket,
		schema.TaskIdPendingPoolBucket,
		schema.TaskBucket,
		schema.BundleItemBinary,
		schema.BundleItemMeta,
		schema.BundleWaitParseArIdBucket,
		schema.BundleArIdToItemIdsBucket,
	}

	ownBuckets, err := getBucketWithPrefix(svc, prefix)
	if err != nil {
		return err
	}

	for _, bucketName := range bucketNames {
		s3Bkt := getS3Bucket(prefix, bucketName) // s3 bucket name only accept lower case
		if !ownBuckets[s3Bkt] {
			err := svc.CreateBucket(s3Bkt)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func getBucketWithPrefix(svc *oss.Client, prefix string) (map[string]bool, error) {
	res := make(map[string]bool)

	lsRes, err := svc.ListBuckets(oss.Prefix(prefix))
	if err != nil {
		return nil, err
	}

	for _, bucket := range lsRes.Buckets {
		res[bucket.Name] = true
	}

	return res, nil
}

func handleOSSErr(ossErr error) (err error) {
	switch ossErr.(type) {
	case oss.ServiceError:
		if ossErr.(oss.ServiceError).Code == ossErrorNoSuchKey {
			err = schema.ErrNotExist
		}
	default:
		err = ossErr
	}

	return
}
