package mystore

import (
	"context"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/datastore"
)

type gcloudStore[T any] struct {
	client *datastore.Client
	kind   string
}

func newGcloudStore[T any](c context.Context) (*gcloudStore[T], func(), error) {
	projectId := os.Getenv("GOOGLE_CLOUD_PROJECT")
	client, err := datastore.NewClient(c, projectId)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating datastore-client: %s", err)
	}

	val := new(T)
	kind := strings.Split(fmt.Sprintf("%T", *val), ".")[1]

	return &gcloudStore[T]{
			client: client,
			kind:   kind,
		}, func() {
			client.Close()
		}, nil
}

func (s *gcloudStore[T]) RunInTransaction(c context.Context, f func(c context.Context) error) error {
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

func (s *gcloudStore[T]) Put(c context.Context, uid string, value T) error {
	transaction := c.Value("transaction")

	if transaction != nil {
		_, err := transaction.(*datastore.Transaction).Put(datastore.NameKey(s.kind, uid, nil), value)
		if err != nil {
			return fmt.Errorf("error transctionally storing entity %s with uid %s: %s", s.kind, uid, err)
		}
		return nil
	}

	_, err := s.client.Put(c, datastore.NameKey(s.kind, uid, nil), &value)
	if err != nil {
		return fmt.Errorf("error storing entity %s with uid %s: %s", s.kind, uid, err)
	}
	return nil
}

func (s *gcloudStore[T]) Get(c context.Context, uid string) (T, bool, error) {
	value := new(T)

	transaction := c.Value("transaction")

	if transaction != nil {
		err := transaction.(*datastore.Transaction).Get(datastore.NameKey(s.kind, uid, nil), value)
		if err != nil {
			if err == datastore.ErrNoSuchEntity {
				return *value, false, nil
			}
			return *value, false, fmt.Errorf("error transctionally fetching entity %s with uid %s: %s", s.kind, uid, err)
		}
		return *value, true, nil
	}

	err := s.client.Get(c, datastore.NameKey(s.kind, uid, nil), value)
	if err != nil {
		if err == datastore.ErrNoSuchEntity {
			return *value, false, nil
		}
		return *value, false, fmt.Errorf("error fetching entity %s with uid %s: %s", s.kind, uid, err)
	}
	return *value, true, nil
}

func (s *gcloudStore[T]) List(c context.Context) ([]T, error) {
	objectsToFetch := []T{}

	// not transactional for now

	q := datastore.NewQuery(s.kind)

	_, err := s.client.GetAll(c, q, &objectsToFetch)
	if err != nil {
		return nil, fmt.Errorf("error fetching all entities %s: %s", s.kind, err)
	}
	return objectsToFetch, nil
}
