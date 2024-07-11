package termsconditions

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/MarcGrol/shopbackend/lib/mycontext"
	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/MarcGrol/shopbackend/lib/myhttp"
	"github.com/MarcGrol/shopbackend/lib/mylog"
	"github.com/MarcGrol/shopbackend/lib/mypublisher"
)

type webService struct {
	logger    mylog.Logger
	publisher mypublisher.Publisher
}

// Use dependency injection to isolate the infrastructure and ease testing
func NewService(pub mypublisher.Publisher) *webService {
	logger := mylog.New("termsconditions")

	return &webService{
		logger:    logger,
		publisher: pub,
	}
}

func (s *webService) RegisterEndpoints(c context.Context, router *mux.Router) error {
	router.HandleFunc("/termsconditions", s.getTermsAndConditions()).Methods("GET")
	router.HandleFunc("/termsconditions", s.acceptTermsAndConditions()).Methods("POST")

	return s.Subscribe(c)
}

func (s *webService) Subscribe(c context.Context) error {
	err := s.publisher.CreateTopic(c, TopicName)
	if err != nil {
		return fmt.Errorf("error creating topic %s: %s", TopicName, err)
	}

	return nil
}

//go:embed templates
var templateFolder embed.FS
var (
	termsConditionsPageTemplate *template.Template
)

func init() {
	termsConditionsPageTemplate = template.Must(template.ParseFS(templateFolder, "templates/termsconditions.html"))
}

func (s *webService) getTermsAndConditions() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		responseWriter := myhttp.NewWriter(s.logger)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		err := termsConditionsPageTemplate.Execute(w, nil)
		if err != nil {
			responseWriter.WriteError(c, w, 1, myerrors.NewInternalError(err))
			return
		}
	}
}

func (s *webService) acceptTermsAndConditions() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		responseWriter := myhttp.NewWriter(s.logger)

		err := s.publisher.Publish(c, TopicName, TermsConditionsAccepted{
			EmailAddress: "lala",
			Version:      "0.0.1",
		})
		if err != nil {
			responseWriter.WriteError(c, w, 2, err)
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)

	}
}
