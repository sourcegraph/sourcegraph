package database

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/encryption"
)

func TestRecordEncrypter(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	key := &base64Key{}
	encrypter := NewRecordEncrypter(db)

	if err := encrypter.Exec(ctx, sqlf.Sprintf("CREATE TABLE test_encryptable (id int, encryption_key_id text, data text)")); err != nil {
		t.Fatalf("failed to create test table: %s", err)
	}

	var writtenValues []string
	var encodedValues []string
	for i := range 20 {
		var data *string
		if i%6 == 0 {
			data = nil
		} else {
			payload := fmt.Sprintf("data-%02d", i)
			data = &payload
		}

		if err := encrypter.Exec(ctx, sqlf.Sprintf("INSERT INTO test_encryptable VALUES (%s, '', %s)", i+1, data)); err != nil {
			t.Fatalf("failed to insert test data: %s", err)
		}

		writtenValues = append(writtenValues, unwrap(data))
		encodedValues = append(encodedValues, base64.StdEncoding.EncodeToString([]byte(unwrap(data))))
	}
	sort.Strings(writtenValues)
	sort.Strings(encodedValues)

	config := EncryptionConfig{
		TableName:           "test_encryptable",
		IDFieldName:         "id",
		KeyIDFieldName:      "encryption_key_id",
		EncryptedFieldNames: []string{"data"},
		Scan:                basestore.NewMapScanner(scanNullableEncryptedString),
		TreatEmptyAsNull:    true,
		Key:                 func() encryption.Key { return key },
		Limit:               5,
	}

	// Encode data in chunks
	for i := range 4 {
		count, err := encrypter.EncryptBatch(ctx, config)
		if err != nil {
			t.Fatalf("unexpected error encrypting batch: %s", err)
		}
		if count != 5 {
			t.Errorf("unexpected count. want=%d have=%d", 5, count)
		}

		numEncrypted, numUnencrypted, err := encrypter.Count(ctx, config)
		if err != nil {
			t.Fatalf("unexpected error counting records: %s", err)
		}
		if want := 5 * (i + 1); numEncrypted != want {
			t.Errorf("unexpected numEncrypted. have=%d want=%d", want, numEncrypted)
		}
		if want := 20 - 5*(i+1); numUnencrypted != want {
			t.Errorf("unexpected numEncrypted. have=%d want=%d", want, numUnencrypted)
		}
	}

	// Expect data to be encoded
	data, err := basestore.ScanNullStrings(encrypter.Query(ctx, sqlf.Sprintf("SELECT data FROM test_encryptable ORDER BY data NULLS FIRST")))
	if err != nil {
		t.Fatalf("failed to query data: %s", err)
	}
	if diff := cmp.Diff(encodedValues, data); diff != "" {
		t.Errorf("unexpected data (-want +got):\n%s", diff)
	}
	encryptionKeyIDs, err := basestore.ScanStrings(encrypter.Query(ctx, sqlf.Sprintf("SELECT encryption_key_id FROM test_encryptable")))
	if err != nil {
		t.Fatalf("failed to query encryption keys: %s", err)
	}
	for _, keyID := range encryptionKeyIDs {
		if want := testEncryptionKeyID(key); keyID != want {
			t.Errorf("unexpected key identifier. want=%q have=%q", want, keyID)
		}
	}

	// Decode data in chunks
	for i := range 4 {
		count, err := encrypter.DecryptBatch(ctx, config)
		if err != nil {
			t.Fatalf("unexpected error decrypting batch: %s", err)
		}
		if count != 5 {
			t.Errorf("unexpected count. want=%d have=%d", 5, count)
		}

		numEncrypted, numUnencrypted, err := encrypter.Count(ctx, config)
		if err != nil {
			t.Fatalf("unexpected error counting records: %s", err)
		}
		if want := 20 - 5*(i+1); numEncrypted != want {
			t.Errorf("unexpected numEncrypted. have=%d want=%d", want, numEncrypted)
		}
		if want := 5 * (i + 1); numUnencrypted != want {
			t.Errorf("unexpected numEncrypted. have=%d want=%d", want, numUnencrypted)
		}
	}

	// Expect data to be decoded
	data, err = basestore.ScanNullStrings(encrypter.Query(ctx, sqlf.Sprintf("SELECT data FROM test_encryptable ORDER BY data NULLS FIRST")))
	if err != nil {
		t.Fatalf("failed to query data: %s", err)
	}
	if diff := cmp.Diff(writtenValues, data); diff != "" {
		t.Errorf("unexpected data (-want +got):\n%s", diff)
	}
	encryptionKeyIDs, err = basestore.ScanStrings(encrypter.Query(ctx, sqlf.Sprintf("SELECT encryption_key_id FROM test_encryptable")))
	if err != nil {
		t.Fatalf("failed to query encryption keys: %s", err)
	}
	for _, keyID := range encryptionKeyIDs {
		if keyID != "" {
			t.Errorf("unexpected key identifier. want=%q have=%q", "", keyID)
		}
	}
}

type base64Key struct{}

func (k *base64Key) Version(ctx context.Context) (encryption.KeyVersion, error) {
	return encryption.KeyVersion{
		Type:    "base64",
		Name:    "base64",
		Version: "0-test",
	}, nil
}

func (k *base64Key) Encrypt(ctx context.Context, value []byte) ([]byte, error) {
	return []byte(base64.StdEncoding.EncodeToString(value)), nil
}

func (k *base64Key) Decrypt(ctx context.Context, cipherText []byte) (*encryption.Secret, error) {
	text, err := base64.StdEncoding.DecodeString(string(cipherText))
	if err != nil {
		return nil, err
	}

	secret := encryption.NewSecret(string(text))
	return &secret, nil
}

func unwrap(v *string) string {
	if v == nil {
		return ""
	}

	return *v
}
