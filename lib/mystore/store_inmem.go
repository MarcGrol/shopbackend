package mystore

import (
	"context"
	"log"
	"sync"
)

type InMemoryStore[T any] struct {
	sync.Mutex
	Items map[string]T
}

func NewInMemoryStore[T any](c context.Context) (*InMemoryStore[T], func(), error) {
	return &InMemoryStore[T]{
		Items: make(map[string]T),
	}, func() {}, nil
}

func (s *InMemoryStore[T]) RunInTransaction(c context.Context, f func(c context.Context) error) error {
	// Start transaction
	s.Lock()

	ctx := context.WithValue(c, ctxTransactionKey{}, true)

	// Within this block everything is transactional
	log.Printf("Func %p with context %p", f, ctx)
	err := f(ctx)
	if err != nil {

		// Rollback
		s.Unlock()

		return err
	}

	// Commit
	s.Unlock()

	return nil
}

func (s *InMemoryStore[T]) Put(c context.Context, uid string, value T) error {
	nonTransactional := c.Value(ctxTransactionKey{}) == nil

	if nonTransactional {
		s.Lock()
	}

	s.Items[uid] = value

	if nonTransactional {
		s.Unlock()
	}

	return nil
}

func (s *InMemoryStore[T]) Get(c context.Context, uid string) (T, bool, error) {
	nonTransactional := c.Value(ctxTransactionKey{}) == nil

	if nonTransactional {
		s.Lock()
	}
	result, exists := s.Items[uid]

	if nonTransactional {
		s.Unlock()
	}

	return result, exists, nil
}

func (s *InMemoryStore[T]) List(c context.Context) ([]T, error) {
	nonTransactional := c.Value(ctxTransactionKey{}) == nil

	if nonTransactional {
		s.Lock()
	}

	result := make([]T, 0, len(s.Items))
	for _, v := range s.Items {
		result = append(result, v)
	}

	if nonTransactional {
		s.Unlock()
	}

	return result, nil
}

func (s *InMemoryStore[T]) Query(c context.Context, filters []Filter, orderByField string) ([]T, error) {
	return s.List(c)
}
