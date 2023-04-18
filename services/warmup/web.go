package warmup

import (
	"context"
	"github.com/MarcGrol/shopbackend/lib/myvault"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/MarcGrol/shopbackend/lib/mycontext"
	"github.com/MarcGrol/shopbackend/lib/myhttp"
	"github.com/MarcGrol/shopbackend/lib/mylog"
)

type webService struct {
	logger mylog.Logger
	vault  myvault.VaultReader
}

// Use dependency injection to isolate the infrastructure and ease testing
func NewService(vault myvault.VaultReader) *webService {
	logger := mylog.New("basket")
	return &webService{
		logger: logger,
		vault:  vault,
	}
}

func (s webService) RegisterEndpoints(c context.Context, router *mux.Router) {
	router.HandleFunc("/_ah/warmup", s.warmupPage()).Methods("GET")
}

func (s *webService) warmupPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := mycontext.ContextFromHTTPRequest(r)
		errorWriter := myhttp.NewWriter(s.logger)

		_, _, err := s.vault.Get(c, myvault.CurrentToken)
		if err != nil {
			errorWriter.WriteError(c, w, 1, err)
			return
		}

		errorWriter.Write(c, w, http.StatusOK, myhttp.SuccessResponse{
			Message: "Successfully processed warmup request",
		})
	}
}
