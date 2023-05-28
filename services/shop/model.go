package shop

import (
	"fmt"
	"net/url"
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
	TotalPrice             int
	Currency               string
	SelectedProducts       []SelectedProduct
	PaymentServiceProvider string
	InitialPaymentStatus   string
	CheckoutStatus         string
	CheckoutStatusDetails  string
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
		lines = append(lines, fmt.Sprintf("%d x	 %s", p.Quantity, p.Description))
	}

	return strings.Join(lines, ", ")
}

func (b Basket) IsNotPaid() bool {
	return !b.IsPaid()
}

func (b Basket) IsPaid() bool {
	return strings.Contains(b.GetPaymentStatus(), "success")
}

func (b Basket) GetPaymentStatus() string {
	if b.CheckoutStatus != "" {
		return fmt.Sprintf("%s (%s)", b.CheckoutStatus, b.CheckoutStatusDetails)
	}

	return b.InitialPaymentStatus
}

type SelectedProduct struct {
	UID         string
	Description string
	Price       int
	Currency    string
	Quantity    int
}

func (p SelectedProduct) TotalPrice() int {
	return int(p.Price) * p.Quantity
}

type BasketDetailPageInfo struct {
	Basket     Basket
	FormValues url.Values
}
