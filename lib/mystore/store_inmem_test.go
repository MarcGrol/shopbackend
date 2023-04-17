package mystore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Person struct {
	UID  string
	Name string
	Age  int
}

var (
	person = Person{UID: "123", Name: "Marc", Age: 42}
)

func TestStore(t *testing.T) {
	c := context.TODO()
	ps, cleanup, err := newInMemoryStore[Person](c)
	assert.NoError(t, err)
	defer cleanup()

	t.Run("Get not found", func(t *testing.T) {
		_, found, err := ps.Get(c, person.UID)
		assert.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("Get put", func(t *testing.T) {
		err = ps.Put(c, person.UID, person)
		assert.NoError(t, err)
	})

	t.Run("Get found", func(t *testing.T) {
		p, found, err := ps.Get(c, person.UID)
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, Person{UID: "123", Name: "Marc", Age: 42}, p)
	})

	t.Run("List", func(t *testing.T) {
		all, err := ps.List(c)
		assert.NoError(t, err)
		assert.Equal(t, all, []Person{person})
	})

}
