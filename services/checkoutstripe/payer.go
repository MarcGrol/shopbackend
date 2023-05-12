package checkoutstripe

import (
	"context"
	"fmt"

	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/stripe/stripe-go/v74"
	"github.com/stripe/stripe-go/v74/checkout/session"
)

//go:generate mockgen -source=payer.go -package checkoutstripe -destination payer_mock.go Payer
type Payer interface {
	UseApiKey(key string)
	UseToken(accessToken string)
	CreateCheckoutSession(ctx context.Context, params stripe.CheckoutSessionParams) (stripe.CheckoutSession, error)
}

type stripePayer struct{}

func NewPayer() Payer {
	return &stripePayer{}
}

func (p *stripePayer) UseApiKey(apiKey string) {
	stripe.Key = apiKey
}

func (p *stripePayer) UseToken(accessToken string) {
	stripe.Key = accessToken
}

func (p *stripePayer) CreateCheckoutSession(ctx context.Context, params stripe.CheckoutSessionParams) (stripe.CheckoutSession, error) {
	session, err := session.New(&params)
	if err != nil {
		return stripe.CheckoutSession{}, myerrors.NewInvalidInputError(fmt.Errorf("error creating stripe session: %s", err))
	}

	return *session, nil
}
