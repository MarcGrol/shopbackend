package mystore

import (
	"context"
	"fmt"
	"github.com/MarcGrol/shopbackend/lib/mycontext"
	"net/http"

	"github.com/gorilla/mux"
)

type Person struct {
	Name string
	Age  int
}

type PersonService struct {
	store Store[Person]
}

func NewPersonService(store Store[Person]) *PersonService {
	return &PersonService{
		store: store,
	}
}

func (s PersonService) RegisterEndpoints(c context.Context, router *mux.Router) {
	router.HandleFunc("/experiment", s.doitWeb()).Methods("GET")
}

func (s *PersonService) doitWeb() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)

		err := s.Doit(c)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "OK")
	}
}

func (s *PersonService) Doit(c context.Context) error {

	fmt.Printf("Starting\n")
	err := s.store.RunInTransaction(c, func(c context.Context) error {

		fmt.Printf("Inside\n")

		person := Person{
			Name: "Marc",
			Age:  42,
		}
		err := s.store.Put(c, "123", person)
		if err != nil {
			fmt.Printf("Error creating: %v\n", err)
			return err
		}
		fmt.Printf("Created: %v\n", person)

		found, exists, err := s.store.Get(c, "123")
		if err != nil {
			fmt.Printf("Error getting: %v\n", err)
			return err
		}
		if !exists {
			fmt.Printf("Not found\n")
			return fmt.Errorf("not found")
		}
		fmt.Printf("Found: %+v\n", found)

		all, err := s.store.List(c)
		if err != nil {
			fmt.Printf("Error fetching: %v\n", err)
			return err
		}
		fmt.Printf("All: %v\n", all)

		return nil
	})
	if err != nil {
		return fmt.Errorf("Error running transaction: %v", err)
	}
	fmt.Printf("Done\n")

	return nil
}
