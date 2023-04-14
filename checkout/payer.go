package checkout

import (
	"context"
	"fmt"
	"strings"

	"github.com/adyen/adyen-go-api-library/v6/src/adyen"
	"github.com/adyen/adyen-go-api-library/v6/src/checkout"
	"github.com/adyen/adyen-go-api-library/v6/src/common"
)

//go:generate mockgen -source=payer.go -package checkout -destination payer_mock.go Payer
type Payer interface {
	UseApiKey(key string)
	UseToken(accessToken string)
	Sessions(ctx context.Context, req checkout.CreateCheckoutSessionRequest) (checkout.CreateCheckoutSessionResponse, error)
	PaymentMethods(ctx context.Context, req checkout.PaymentMethodsRequest) (checkout.PaymentMethodsResponse, error)
}

type adyenPayer struct {
	client *adyen.APIClient
}

func NewPayer(environment string, apiKey string) Payer {
	return &adyenPayer{
		client: adyen.NewClient(&common.Config{
			ApiKey:      apiKey,
			Environment: common.Environment(strings.ToUpper(environment)),
			Debug:       false,
		}),
	}
}

func (p *adyenPayer) UseApiKey(apiKey string) {
	// clear header
	delete(p.client.GetConfig().DefaultHeader, "Authorization")
	// set api-key
	p.client.GetConfig().ApiKey = apiKey
}

func (p *adyenPayer) UseToken(accessToken string) {
	// clear api-key
	p.client.GetConfig().ApiKey = ""
	// set header
	p.client.GetConfig().DefaultHeader["Authorization"] = fmt.Sprintf("Bearer %s", accessToken)
}

func (p *adyenPayer) Sessions(ctx context.Context, req checkout.CreateCheckoutSessionRequest) (checkout.CreateCheckoutSessionResponse, error) {
	resp, _, err := p.client.Checkout.Sessions(&req, ctx)
	if err != nil {
		return checkout.CreateCheckoutSessionResponse{}, err
	}
	return resp, err
}

func (p *adyenPayer) PaymentMethods(ctx context.Context, req checkout.PaymentMethodsRequest) (checkout.PaymentMethodsResponse, error) {
	resp, _, err := p.client.Checkout.PaymentMethods(&req, ctx)
	if err != nil {
		return checkout.PaymentMethodsResponse{}, err
	}
	return resp, err
}
