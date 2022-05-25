package storage_test

import (
	"testing"

	"github.com/alcionai/corso/pkg/storage"
)

func TestS3Config_Config(t *testing.T) {
	s3 := storage.S3Config{"bkt", "ak", "sk"}
	c := s3.Config()
	table := []struct {
		key    string
		expect string
	}{
		{"s3_bucket", s3.Bucket},
		{"s3_accessKey", s3.AccessKey},
		{"s3_secretKey", s3.SecretKey},
	}
	for _, test := range table {
		key := test.key
		expect := test.expect
		if c[key] != expect {
			t.Errorf("expected config key [%s] to hold value [%s], got [%s]", key, expect, c[key])
		}
	}
}

func TestStorage_S3Config(t *testing.T) {
	in := storage.S3Config{"bkt", "ak", "sk"}
	s := storage.NewStorage(storage.ProviderS3, in)
	out := s.S3Config()
	if in.Bucket != out.Bucket {
		t.Errorf("expected S3Config.Bucket to be [%s], got [%s]", in.Bucket, out.Bucket)
	}
	if in.AccessKey != out.AccessKey {
		t.Errorf("expected S3Config.AccessKey to be [%s], got [%s]", in.AccessKey, out.AccessKey)
	}
	if in.SecretKey != out.SecretKey {
		t.Errorf("expected S3Config.SecretKey to be [%s], got [%s]", in.SecretKey, out.SecretKey)
	}
}
