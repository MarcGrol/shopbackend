package mystore

import (
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/datastore"
)

type gcloudDataStore struct {
	client *datastore.Client
}

func NewStore(c context.Context) (DataStorer, func(), error) {
	projectId := os.Getenv("GOOGLE_CLOUD_PROJECT")
	client, err := datastore.NewClient(c, projectId)
	if err != nil {
		return nil, nil, fmt.Errorf("Error creating datastore-client: %s", err)
	}
	return &gcloudDataStore{
			client: client,
		}, func() {
			client.Close()
		}, nil
}

func (s *gcloudDataStore) Put(c context.Context, kind, uid string, objectToStore interface{}) error {
	_, err := s.client.Put(c, datastore.NameKey(kind, uid, nil), objectToStore)
	if err != nil {
		return fmt.Errorf("Error storing entity %s with uid %s: %s", kind, uid, err)
	}
	return nil
}

func (s *gcloudDataStore) Get(c context.Context, kind, uid string, objectToFetch interface{}) (bool, error) {
	err := s.client.Get(c, datastore.NameKey(kind, uid, nil), objectToFetch)
	if err != nil {
		if err == datastore.ErrNoSuchEntity {
			return false, nil
		}
		return false, fmt.Errorf("Error fetching entity %s with uid %s: %s", kind, uid, err)
	}
	return true, nil
}

func (s *gcloudDataStore) List(c context.Context, kind string, objectsToFetch interface{}) error {
	q := datastore.NewQuery(kind)
	_, err := s.client.GetAll(c, q, objectsToFetch)
	if err != nil {
		return fmt.Errorf("Error fetching all entities %s: %s", kind, err)
	}
	return nil
}
