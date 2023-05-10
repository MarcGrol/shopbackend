package checkoutapi

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	formcodec "github.com/go-playground/form/v4"

	"github.com/MarcGrol/shopbackend/lib/myerrors"
)

type Checkout struct {
	BasketUID    string    `form:"basketUid"`
	TotalAmount  Amount    `form:"amount"`
	Company      Company   `form:"company"`
	Shopper      Shopper   `form:"shopper"`
	ProductCount int       `form:"product.count"`
	Products     []Product `form:"products"`
	ReturnURL    string    `form:"returnUrl"`
}

type Company struct {
	Name        string `form:"name"`
	Homepage    string `form:"homepage"`
	CountryCode string `form:"countryCode"`
	ShopName    string `form:"shopName"`
}

type Amount struct {
	Amount   int    `form:"amount"`
	Currency string `form:"currency"`
}

type Shopper struct {
	UID         string      `form:"uid"`
	Locale      string      `form:"locale"`
	FirstName   string      `form:"firstName"`
	LastName    string      `form:"lastName"`
	ContactInfo ContactInfo `form:"contactInfo"`
	Address     Address     `form:"address"`
}

type ContactInfo struct {
	PhoneNumber string `form:"phone"`
	Email       string `form:"email"`
}

type Address struct {
	Street             string `form:"street"`
	AddressHouseNumber string `form:"houseNumber"`
	PostalCode         string `form:"postalCode"`
	City               string `form:"city"`
	State              string `form:"state"`
	Country            string `form:"country"`
}

type Product struct {
	Name        string `form:"name"`
	Description string `form:"description"`
	ItemPrice   int    `form:"itemPrice"`
	Currency    string `form:"currency"`
	Quantity    int    `form:"quantity"`
	TotalPrice  int    `form:"totalPrice"`
}

func NewFromRequest(r *http.Request) (Checkout, error) {
	err := r.ParseForm()
	if err != nil {
		return Checkout{}, myerrors.NewInvalidInputError(err)
	}
	return NewFromValues(r.Form)
}

func NewFromValues(values url.Values) (Checkout, error) {
	checkout := Checkout{}
	err := formcodec.NewDecoder().Decode(&checkout, values)
	if err != nil {
		return checkout, fmt.Errorf("error decoding form: %s", err)
	}

	return checkout, nil
}

func (c Checkout) ToForm() (url.Values, error) {
	values, err := formcodec.NewEncoder().Encode(c)
	if err != nil {
		return nil, fmt.Errorf("error decoding form: %s", err)
	}

	return values, nil
}

func FormValuesToHtml(values url.Values) string {
	buf := strings.Builder{}
	for key, value := range values {
		buf.WriteString(fmt.Sprintf("<input type=\"hidden\" name=\"%s\" value=\"%s\"/>\n", key, value[0]))
	}
	return buf.String()
}
