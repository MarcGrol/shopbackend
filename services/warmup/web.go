package warmup

import (
	"context"
	"fmt"
	"net/http"

	"github.com/MarcGrol/shopbackend/lib/mypublisher"
	"github.com/MarcGrol/shopbackend/lib/myuuid"
	"github.com/MarcGrol/shopbackend/lib/myvault"
	"github.com/MarcGrol/shopbackend/services/oauth/oauthvault"

	"github.com/gorilla/mux"

	"github.com/MarcGrol/shopbackend/lib/mycontext"
	"github.com/MarcGrol/shopbackend/lib/myhttp"
	"github.com/MarcGrol/shopbackend/lib/mylog"
)

type webService struct {
	logger    mylog.Logger
	vault     myvault.VaultReader[oauthvault.Token]
	uider     myuuid.UUIDer
	publisher mypublisher.Publisher
}

// Use dependency injection to isolate the infrastructure and ease testing
func NewService(vault myvault.VaultReader[oauthvault.Token], uider myuuid.UUIDer, pub mypublisher.Publisher) *webService {
	logger := mylog.New("basket")

	return &webService{
		logger:    logger,
		vault:     vault,
		publisher: pub,
		uider:     uider,
	}
}

func (s webService) RegisterEndpoints(c context.Context, router *mux.Router) error {
	router.HandleFunc("/_ah/warmup", s.warmupPage()).Methods("GET")

	return s.Subscribe(c)
}

func (s *webService) Subscribe(c context.Context) error {
	err := s.publisher.CreateTopic(c, TopicName)
	if err != nil {
		return fmt.Errorf("error creating topic %s: %s", TopicName, err)
	}

	return nil
}

func (s *webService) warmupPage() http.HandlerFunc {
	uid := s.uider.Create()

	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		_, _, err := s.vault.Get(c, oauthvault.CurrentToken)
		if err != nil {
			errorWriter.WriteError(c, w, 1, err)
			return
		}

		err = s.publisher.Publish(c, TopicName, WarmupKicked{
			UID: uid,
		})
		if err != nil {
			errorWriter.WriteError(c, w, 2, err)
		}

		errorWriter.Write(c, w, http.StatusOK, myhttp.SuccessResponse{
			Message: "Successfully processed warmup request",
		})
	}
}
