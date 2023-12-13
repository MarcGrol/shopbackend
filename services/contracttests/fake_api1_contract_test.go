package contracttests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInMemoryAPI1(t *testing.T) {
	API1Contract{
		api1: func() API1 {
			return NewFakeApi1()
		},
	}.Test(t)
}

type API1Contract struct {
	api1 func() API1
}

func (c API1Contract) Test(t *testing.T) {
	t.Run("can create, get and update a customer", func(t *testing.T) {
		var (
			sut  = c.api1()
			ctx  = context.Background()
			name = "Bob"
		)

		customer, err := sut.CreateCustomer(ctx, name)
		assert.NoError(t, err)

		got, err := sut.GetCustomer(ctx, customer.ID)
		assert.NoError(t, err)
		assert.Equal(t, customer, got)

		newName := "Robert"
		assert.NoError(t, sut.UpdateCustomer(ctx, customer.ID, newName))

		got, err = sut.GetCustomer(ctx, customer.ID)
		assert.NoError(t, err)
		assert.Equal(t, newName, got.Name)
	})

	t.Run("can recognize when a customer does not exist", func(t *testing.T) {
		var (
			sut = c.api1()
			ctx = context.Background()
			id  = "123"
		)

		_, err := sut.GetCustomer(ctx, id)
		assert.Equal(t, ErrCustomerDoesNorExist, err)

		err = sut.UpdateCustomer(ctx, id, "Bob")
		assert.Equal(t, ErrCustomerDoesNorExist, err)

	})

	// example of strange behaviours we didn't expect
	t.Run("the system will not allow you to add 'Dave' as a customer", func(t *testing.T) {
		var (
			sut  = c.api1()
			ctx  = context.Background()
			name = "Dave"
		)

		_, err := sut.CreateCustomer(ctx, name)
		assert.Equal(t, "customer Dave is forbidden", err.Error())
	})
}
