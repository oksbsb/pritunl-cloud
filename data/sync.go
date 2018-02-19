package data

import (
	"crypto/md5"
	"fmt"
	"github.com/dropbox/godropbox/container/set"
	"github.com/dropbox/godropbox/errors"
	"github.com/minio/minio-go"
	"github.com/pritunl/pritunl-cloud/database"
	"github.com/pritunl/pritunl-cloud/errortypes"
	"github.com/pritunl/pritunl-cloud/image"
	"github.com/pritunl/pritunl-cloud/storage"
	"regexp"
	"time"
)

var etagReg = regexp.MustCompile("[^a-zA-Z0-9]+")

func Sync(db *database.Database, store *storage.Storage) (err error) {
	client, err := minio.New(
		store.Endpoint, store.AccessKey, store.SecretKey, !store.Insecure)
	if err != nil {
		err = &errortypes.ConnectionError{
			errors.New("storage: Failed to connect to storage"),
		}
		return
	}

	done := make(chan struct{})
	defer close(done)

	remoteKeys := set.NewSet()
	for object := range client.ListObjects(store.Bucket, "", true, done) {
		if object.Err != nil {
			err = &errortypes.RequestError{
				errors.New("storage: Failed to list objects"),
			}
			return
		}

		etag := object.ETag
		if etag == "" {
			modifiedHash := md5.New()
			modifiedHash.Write(
				[]byte(object.LastModified.Format(time.RFC3339)))
			etag = fmt.Sprintf("%x", modifiedHash.Sum(nil))
		}

		etag = etagReg.ReplaceAllString(etag, "")

		remoteKeys.Add(object.Key)

		img := &image.Image{
			Storage: store.Id,
			Key:     object.Key,
			Etag:    etag,
			Type:    store.Type,
		}
		err = img.Upsert(db)
		if err != nil {
			return
		}
	}

	localKeys, err := image.Distinct(db, store.Id)
	if err != nil {
		return
	}

	removeKeysSet := set.NewSet()
	for _, key := range localKeys {
		removeKeysSet.Add(key)
	}
	removeKeysSet.Subtract(remoteKeys)

	removeKeys := []string{}
	for key := range removeKeysSet.Iter() {
		removeKeys = append(removeKeys, key.(string))
	}

	err = image.RemoveKeys(db, store.Id, removeKeys)
	if err != nil {
		return
	}

	return
}