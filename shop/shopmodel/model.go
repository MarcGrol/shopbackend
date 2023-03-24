package shopmodel

import (
	"fmt"
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

type Basket struct {
	UID                  string
	CreatedAt            time.Time
	Shop                 Shop
	Shopper              Shopper
	TotalPrice           int64
	Currency             string
	SelectedProducts     []SelectedProduct
	InitialPaymentStatus string
	FinalPaymentEvent    string
	FinalPaymentStatus   string
	ReturnURL            string
}

func (b Basket) Timestamp() string {
	return b.CreatedAt.Format("2006-01-02 15:04:05")
}

func (b Basket) GetPriceInCurrency() string {
	return fmt.Sprintf("%s %.2f", b.Currency, float32(b.TotalPrice/100.00))
}

func (b Basket) IsNotPaid() bool {
	return b.InitialPaymentStatus == "open" || b.InitialPaymentStatus == "error" || b.InitialPaymentStatus == "failed"
}

type SelectedProduct struct {
	UID         string
	Description string
	Price       int64
	Currency    string
	Quantity    int
}
