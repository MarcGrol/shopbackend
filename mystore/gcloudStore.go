package mystore

import (
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/datastore"
)

type gcloudDataStore struct {
	client      *datastore.Client
	typeCreator func() (interface{}, interface{})
}

func init() {
	if os.Getenv("GOOGLE_CLOUD_PROJECT") != "" {
		New = newCloudStore
	}
}

func newCloudStore(c context.Context, typeCreator func() (interface{}, interface{})) (DataStorer, func(), error) {
	projectId := os.Getenv("GOOGLE_CLOUD_PROJECT")
	client, err := datastore.NewClient(c, projectId)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating datastore-client: %s", err)
	}
	return &gcloudDataStore{
			client:      client,
			typeCreator: typeCreator,
		}, func() {
			client.Close()
		}, nil
}

func (s *gcloudDataStore) RunInTransaction(c context.Context, f func(c context.Context) error) error {
	// Start transaction
	t, err := s.client.NewTransaction(c)
	if err != nil {
		return fmt.Errorf("error creating transaction: %s", err)
	}

	ctx := context.WithValue(c, "transaction", t)

	// Within this block everything is transactional
	err = f(ctx)
	if err != nil {

		// Rollback
		err = t.Rollback()
		if err != nil {
			return fmt.Errorf("error rolling-back transaction: %s", err)
		}
		return err
	}

	// Commit
	_, err = t.Commit()
	if err != nil {
		return fmt.Errorf("error committing transaction: %s", err)
	}

	return nil
}

func (s *gcloudDataStore) Put(c context.Context, kind, uid string, objectToStore interface{}) error {
	transaction := c.Value("transaction")

	if transaction != nil {
		_, err := transaction.(*datastore.Transaction).Put(datastore.NameKey(kind, uid, nil), objectToStore)
		if err != nil {
			return fmt.Errorf("error transctionally storing entity %s with uid %s: %s", kind, uid, err)
		}
		return nil
	}

	_, err := s.client.Put(c, datastore.NameKey(kind, uid, nil), objectToStore)
	if err != nil {
		return fmt.Errorf("error storing entity %s with uid %s: %s", kind, uid, err)
	}
	return nil
}

func (s *gcloudDataStore) Get(c context.Context, kind, uid string) (interface{}, bool, error) {
	transaction := c.Value("transaction")

	objectToFetch, _ := s.typeCreator()

	if transaction != nil {
		err := transaction.(*datastore.Transaction).Get(datastore.NameKey(kind, uid, nil), objectToFetch)
		if err != nil {
			if err == datastore.ErrNoSuchEntity {
				return nil, false, nil
			}
			return nil, false, fmt.Errorf("error transctionally fetching entity %s with uid %s: %s", kind, uid, err)
		}
		return objectToFetch, true, nil
	}

	err := s.client.Get(c, datastore.NameKey(kind, uid, nil), objectToFetch)
	if err != nil {
		if err == datastore.ErrNoSuchEntity {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("error fetching entity %s with uid %s: %s", kind, uid, err)
	}
	return objectToFetch, true, nil
}

func (s *gcloudDataStore) List(c context.Context, kind string) (interface{}, error) {
	_, objectsToFetch := s.typeCreator()

	// not transactional for now

	q := datastore.NewQuery(kind)

	_, err := s.client.GetAll(c, q, objectsToFetch)
	if err != nil {
		return nil, fmt.Errorf("error fetching all entities %s: %s", kind, err)
	}
	return objectsToFetch, nil
}
