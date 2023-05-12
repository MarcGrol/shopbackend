package mystore

import (
	"context"
	"fmt"
	"log"
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
	kind := fmt.Sprintf("%T", *val)
	if strings.Contains(kind, ".") {
		kind = strings.Split(kind, ".")[1]
	}

	return &gcloudStore[T]{
			client: client,
			kind:   kind,
		}, func() {
			client.Close()
		}, nil
}

func (s *gcloudStore[T]) RunInTransactionNative(c context.Context, f func(c context.Context) error) error {
	_, err := s.client.RunInTransaction(c, func(tx *datastore.Transaction) error {
		ctx := context.WithValue(c, ctxTransactionKey{}, tx)
		return f(ctx)
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *gcloudStore[T]) RunInTransaction(c context.Context, f func(c context.Context) error) error {
	var err error
	// retry 3 times
	for i := 1; i <= 3; i++ {
		//log.Printf("Attempt %d to run logic transaction", i)
		err = s.runInTransaction(c, f)
		if err != nil {
			if err == datastore.ErrConcurrentTransaction {
				log.Printf("Concurrent transaction error, retrying (%d of %d): %s", i, 3, err)
				// force retry: this approach requires idempotency of the business logic
				continue
			}

			return err
		}
		return nil
	}
	return err
}

func (s *gcloudStore[T]) runInTransaction(c context.Context, f func(c context.Context) error) error {
	// Start transaction
	t, err := s.client.NewTransaction(c)
	if err != nil {
		fmt.Printf("error creating transaction: %s", err)
		return err
	}

	//log.Printf("Start transaction %p", t)

	ctx := context.WithValue(c, ctxTransactionKey{}, t)

	// Shadow original context with new transactional context
	err = f(ctx)
	if err != nil {
		log.Printf("Rolling back transaction %p due to error %s", t, err)

		// Rollback
		rollbackError := t.Rollback()
		if rollbackError != nil {
			fmt.Printf("error rolling-back transaction %p: %s", t, rollbackError)
			return err
		}

		return err
	}

	// Commit
	_, err = t.Commit()
	if err != nil {
		fmt.Printf("error committing transaction %p: %s", t, err)
		return err
	}

	//log.Printf("Committed transaction %p", t)

	return nil
}

func (s *gcloudStore[T]) Put(c context.Context, uid string, value T) error {
	transaction := c.Value(ctxTransactionKey{})

	if transaction != nil {
		_, err := transaction.(*datastore.Transaction).Put(datastore.NameKey(s.kind, uid, nil), &value)
		if err != nil {
			return fmt.Errorf("error transctionally storing entity %s with uid %s: %s", s.kind, uid, err)
		}

		//log.Printf("In transaction %p: stored entity %s with uid %s", transaction, s.kind, uid)

		return nil
	}

	_, err := s.client.Put(c, datastore.NameKey(s.kind, uid, nil), &value)
	if err != nil {
		return fmt.Errorf("error storing entity %s with uid %s: %s", s.kind, uid, err)
	}

	//log.Printf("Non-transactionally stored entity %s with uid %s", s.kind, uid)

	return nil
}

func (s *gcloudStore[T]) Get(c context.Context, uid string) (T, bool, error) {
	value := new(T)

	transaction := c.Value(ctxTransactionKey{})

	if transaction != nil {
		err := transaction.(*datastore.Transaction).Get(datastore.NameKey(s.kind, uid, nil), value)
		if err != nil {
			if err == datastore.ErrNoSuchEntity {
				return *value, false, nil
			}
			return *value, false, fmt.Errorf("error transctionally fetching entity %s with uid %s: %s", s.kind, uid, err)
		}

		//log.Printf("In transaction %p: fetched entity %s with uid %s", transaction, s.kind, uid)

		return *value, true, nil
	}

	err := s.client.Get(c, datastore.NameKey(s.kind, uid, nil), value)
	if err != nil {
		if err == datastore.ErrNoSuchEntity {
			return *value, false, nil
		}
		return *value, false, fmt.Errorf("error fetching entity %s with uid %s: %s", s.kind, uid, err)
	}

	//log.Printf("Non-transactionally fetched entity %s with uid %s", s.kind, uid)

	return *value, true, nil
}

func (s *gcloudStore[T]) List(c context.Context) ([]T, error) {
	transaction := c.Value(ctxTransactionKey{})

	objectsToFetch := []T{}

	q := datastore.NewQuery(s.kind).Limit(100)

	if transaction != nil {
		q = q.Transaction(transaction.(*datastore.Transaction))
	}

	_, err := s.client.GetAll(c, q, &objectsToFetch)
	if err != nil {
		return nil, fmt.Errorf("error fetching all entities %s: %s", s.kind, err)
	}
	return objectsToFetch, nil
}

func (s *gcloudStore[T]) Query(c context.Context, filters []Filter, orderByField string) ([]T, error) {
	objectsToFetch := []T{}

	transaction := c.Value(ctxTransactionKey{})

	q := datastore.NewQuery(s.kind)
	for _, f := range filters {
		q = q.FilterField(f.Field, f.Compare, f.Value)
	}
	q = q.Order(orderByField)

	if transaction != nil {
		q = q.Transaction(transaction.(*datastore.Transaction))
	}
	_, err := s.client.GetAll(c, q, &objectsToFetch)
	if err != nil {
		return nil, fmt.Errorf("error fetching all entities %s: %s", s.kind, err)
	}
	return objectsToFetch, nil
}
