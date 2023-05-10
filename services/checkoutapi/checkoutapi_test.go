package checkoutapi

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeSame(t *testing.T) {
	//  encode followed by decode must end up same

	values, err := checkout.ToForm()
	assert.NoError(t, err)
	checkoutAgain, err := NewFromValues(values)
	assert.NoError(t, err)

	assert.Equal(t, checkout, checkoutAgain)
}

func TestDecode(t *testing.T) {
	form := url.Values{
		"basketUid":                   []string{"123"},
		"returnUrl":                   []string{"http://localhost/basket/123"},
		"company.countryCode":         []string{"NL"},
		"company.homepage":            []string{"https://www.marcgrolconsultancy.nl/"},
		"company.name":                []string{"Evas shop"},
		"company.shopName":            []string{"Evas shop"},
		"shopper.locale":              []string{"nl"},
		"shopper.firstName":           []string{"Marc"},
		"shopper.lastName":            []string{"Grol"},
		"shopper.uid":                 []string{"shopper_marc_grol"},
		"shopper.contactInfo.email":   []string{"marc.grol@gmail.com"},
		"shopper.contactInfo.phone":   []string{"31648928856"},
		"shopper.address.city":        []string{"De Bilt"},
		"shopper.address.country":     []string{"NL"},
		"shopper.address.houseNumber": []string{"79"},
		"shopper.address.postalCode":  []string{"3731TB"},
		"shopper.address.state":       []string{"Utrecht"},
		"shopper.address.street":      []string{"Heemdstrakwartier"},
		"products[0].name":            []string{"product_jogging_pants"},
		"products[0].description":     []string{"Jogging pants"},
		"products[0].itemPrice":       []string{"6000"},
		"products[0].currency":        []string{"EUR"},
		"products[0].quantity":        []string{"1"},
		"products[0].totalPrice":      []string{"6000"},
		"products[1].name":            []string{"product_tennis_racket"},
		"products[1].description":     []string{"Tennis racket"},
		"products[1].itemPrice":       []string{"16900"},
		"products[1].currency":        []string{"EUR"},
		"products[1].quantity":        []string{"2"},
		"products[1].totalPrice":      []string{"23800"},
	}

	checkoutAgain, err := NewFromValues(form)
	assert.NoError(t, err)
	assert.Equal(t, checkout, checkoutAgain)

}

var checkout = Checkout{
	BasketUID: "123",
	ReturnURL: "http://localhost/basket/123",
	Company: Company{
		CountryCode: "NL",
		Homepage:    "https://www.marcgrolconsultancy.nl/",
		Name:        "Evas shop",
		ShopName:    "Evas shop",
	},
	Shopper: Shopper{
		Locale:    "nl",
		FirstName: "Marc",
		LastName:  "Grol",

		UID: "shopper_marc_grol",
		ContactInfo: ContactInfo{
			Email:       "marc.grol@gmail.com",
			PhoneNumber: "31648928856",
		},
		Address: Address{
			City:               "De Bilt",
			Country:            "NL",
			AddressHouseNumber: "79",
			PostalCode:         "3731TB",
			State:              "Utrecht",
			Street:             "Heemdstrakwartier",
		},
	},
	Products: []Product{
		{
			Name:        "product_jogging_pants",
			Description: "Jogging pants",
			ItemPrice:   6000,
			Currency:    "EUR",
			Quantity:    1,
			TotalPrice:  6000,
		},
		{
			Name:        "product_tennis_racket",
			Description: "Tennis racket",
			ItemPrice:   16900,
			Currency:    "EUR",
			Quantity:    2,
			TotalPrice:  23800,
		},
	},
}
