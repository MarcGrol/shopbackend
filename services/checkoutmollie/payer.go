package checkoutmollie

import (
	"context"
	"fmt"

	"github.com/MarcGrol/shopbackend/lib/myerrors"
	"github.com/VictorAvelar/mollie-api-go/v3/mollie"
)

//go:generate mockgen -source=payer.go -package checkoutmollie -destination payer_mock.go Payer
type Payer interface {
	UseAPIKey(key string)
	UseToken(accessToken string)
	CreatePayment(ctx context.Context, request mollie.Payment) (mollie.Payment, error)
	GetPaymentOnID(ctx context.Context, paymentID string) (mollie.Payment, error)
}

type molliePayer struct {
	client *mollie.Client
}

func NewPayer() (Payer, error) {
	config := mollie.NewAPITestingConfig(true)

	client, err := mollie.NewClient(nil, config)
	if err != nil {
		return nil, myerrors.NewInternalError(fmt.Errorf("error creating mollie client: %s", err))
	}

	return &molliePayer{
		client: client,
	}, nil
}

func (p *molliePayer) UseAPIKey(apiKey string) {
	p.client.WithAuthenticationValue(apiKey)
}

func (p *molliePayer) UseToken(accessToken string) {
	p.client.WithAuthenticationValue(accessToken)
	p.client.SetAccessToken(accessToken)
}

func (p *molliePayer) CreatePayment(ctx context.Context, request mollie.Payment) (mollie.Payment, error) {
	_, payment, err := p.client.Payments.Create(ctx, request, nil)
	if err != nil {
		return mollie.Payment{}, myerrors.NewInvalidInputError(fmt.Errorf("error creating mollie payment: %s", err))
	}

	return *payment, nil
}

func (p *molliePayer) GetPaymentOnID(ctx context.Context, id string) (mollie.Payment, error) {
	_, payment, err := p.client.Payments.Get(ctx, id, &mollie.PaymentOptions{})
	if err != nil {
		return mollie.Payment{}, myerrors.NewInvalidInputError(fmt.Errorf("error getting mollie payment: %s", err))
	}

	return *payment, nil
}
