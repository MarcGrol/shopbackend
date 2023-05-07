package shop

import (
	"fmt"
	"strings"
	"time"
)

type Shop struct {
	UID      string
	Name     string
	Country  string
	Currency string
	Hostname string
}

type Shopper struct {
	UID          string
	FirstName    string
	LastName     string
	DateOfBirth  *time.Time
	Address      Address
	Country      string
	EmailAddress string
	Locale       string
	PhoneNumber  string
}

type Address struct {
	City              string
	Country           string
	HouseNumberOrName string
	PostalCode        string
	StateOrProvince   string
	Street            string
}

type BasketState int

const (
	BasketStateIdle BasketState = iota
	BasketStatePaymentCompleted
)

type Basket struct {
	UID                    string
	State                  BasketState
	CreatedAt              time.Time
	LastModified           *time.Time
	Shop                   Shop
	Shopper                Shopper
	TotalPrice             int64
	Currency               string
	SelectedProducts       []SelectedProduct
	PaymentServiceProvider string
	InitialPaymentStatus   string
	FinalPaymentEvent      string
	FinalPaymentStatus     bool
	PaymentMethod          string
	Done                   bool
	ReturnURL              string
}

func (b Basket) Timestamp() string {
	return b.CreatedAt.Format("2006-01-02 15:04:05")
}

func (b Basket) GetPriceInCurrency() string {
	return fmt.Sprintf("%s %.2f", b.Currency, float32(b.TotalPrice/100.00))
}

func (b Basket) GetProductSummary() string {
	lines := []string{}
	for _, p := range b.SelectedProducts {
		lines = append(lines, fmt.Sprintf("%d x %s,", p.Quantity, p.Description))
	}
	return strings.Join(lines, ", ")
}

func (b Basket) IsNotPaid() bool {
	return !b.IsPaid()
}

func (b Basket) IsPaid() bool {
	return b.InitialPaymentStatus == "success" || (b.FinalPaymentEvent == "AUTHORISATION" && b.FinalPaymentStatus)
}

func (b Basket) GetFinalPaymentStatus() string {
	if b.FinalPaymentEvent == "" {
		return ""
	}
	return fmt.Sprintf("%s=%v", b.FinalPaymentEvent, b.FinalPaymentStatus)
}

func (b *Basket) Execute(event any) error {
	switch b.State {

	}
	return nil
}

type SelectedProduct struct {
	UID         string
	Description string
	Price       int64
	Currency    string
	Quantity    int
}
