package contracttests

import (
	"context"
	"errors"

	"github.com/MarcGrol/shopbackend/lib/mystore"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
)

var (
	ErrCustomerDoesNorExist error = errors.New("customer does not exist")
)

type FakeApi1 struct {
	uuider myuuid.RealUUIDer
	Store  *mystore.InMemoryStore[API1Customer]
}

func NewFakeApi1() API1 {
	store, _, _ := mystore.NewInMemoryStore[API1Customer](context.Background())
	return &FakeApi1{
		Store: store,
	}
}

func (a *FakeApi1) CreateCustomer(ctx context.Context, name string) (API1Customer, error) {
	// Dirty exception coded into fake
	if name == "Dave" {
		return API1Customer{}, errors.New("customer Dave is forbidden")
	}

	customer := API1Customer{
		Name: name,
		ID:   a.uuider.Create(),
	}
	err := a.Store.Put(ctx, customer.ID, customer)
	if err != nil {
		return API1Customer{}, err
	}
	return customer, nil
}

func (a *FakeApi1) GetCustomer(ctx context.Context, id string) (API1Customer, error) {
	customer, exists, err := a.Store.Get(ctx, id)
	if err != nil {
		return API1Customer{}, err
	}
	if !exists {
		return API1Customer{}, ErrCustomerDoesNorExist
	}
	return customer, nil
}

func (a *FakeApi1) UpdateCustomer(ctx context.Context, id string, name string) error {
	customer, exists, err := a.Store.Get(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return ErrCustomerDoesNorExist
	}
	customer.Name = name
	err = a.Store.Put(ctx, customer.ID, customer)
	if err != nil {
		return err
	}
	return nil
}
