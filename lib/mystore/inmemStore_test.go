package mystore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIt(t *testing.T) {
	c := context.TODO()
	ps, cleanup, err := newInMemoryStore[Person](c)
	assert.NoError(t, err)
	defer cleanup()

	err = ps.Put(c, "uid", Person{Name: "Marc", Age: 42})
	assert.NoError(t, err)

	p, found, err := ps.Get(c, "uid")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, Person{Name: "Marc", Age: 42}, p)
}
