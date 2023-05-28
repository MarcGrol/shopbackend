package shop

import (
	"fmt"
	"math/rand"
	"time"
)

var r *rand.Rand

func init() {
	r = rand.New(rand.NewSource(time.Now().Unix()))
}

func createBasket(uid string, createdAt time.Time, returnURL string) Basket {
	basket := Basket{
		UID:                  uid,
		CreatedAt:            createdAt,
		Shop:                 getCurrentShop(),
		Shopper:              getCurrentShopper(uid),
		TotalPrice:           0,
		Currency:             "EUR",
		SelectedProducts:     []SelectedProduct{},
		ReturnURL:            returnURL,
		InitialPaymentStatus: "",
	}
	basket.SelectedProducts = append(basket.SelectedProducts, getRandomProduct())
	basket.SelectedProducts = append(basket.SelectedProducts, getRandomProduct())

	basket.TotalPrice = calculateTotalPrice(basket.SelectedProducts)

	return basket
}

func calculateTotalPrice(products []SelectedProduct) int {
	var totalPrice int
	for _, p := range products {
		totalPrice += p.Price * p.Quantity
	}
	return totalPrice
}

func getCurrentShop() Shop {
	return Shop{
		UID:      "shop_evas_shop",
		Name:     "Eva's shop",
		Country:  "NL",
		Currency: "EUR",
		Hostname: "https://www.marcgrolconsultancy.nl/", // "http://localhost:8082"
	}
}

func getCurrentShopper(uid string) Shopper {
	return Shopper{
		UID:         "shopper_marc_grol",
		FirstName:   "Marc",
		LastName:    "Grol",
		DateOfBirth: func() *time.Time { t := time.Date(1971, time.February, 27, 0, 0, 0, 0, time.UTC); return &t }(),
		Address: Address{
			City:              "De Bilt",
			Country:           "NL",
			HouseNumberOrName: "79",
			PostalCode:        "3731TB",
			StateOrProvince:   "Utrecht",
			Street:            "Heemdstrakwartier",
		},
		Country:      "nl",
		Locale:       "nl", //"nl-NL",
		EmailAddress: fmt.Sprintf("marc.grol+%s@gmail.com", uid),
		PhoneNumber:  "+31648928856",
	}
}

func getRandomProduct() SelectedProduct {
	return products[r.Intn(len(products))]
}

var products = []SelectedProduct{
	{
		UID:         "product_hockey_stick",
		Description: "Hockey stick",
		Price:       19000,
		Currency:    "EUR",
		Quantity:    1,
	},
	{
		UID:         "product_hockey_shoes",
		Description: "Hockey shoes",
		Price:       12000,
		Currency:    "EUR",
		Quantity:    1,
	},
	{
		UID:         "product_jogging_pants",
		Description: "Jogging pants",
		Price:       6000,
		Currency:    "EUR",
		Quantity:    1,
	},
	{
		UID:         "product_sweat_shirt",
		Description: "Sweat shirt",
		Price:       7000,
		Currency:    "EUR",
		Quantity:    1,
	},
	{
		UID:         "product_hoody",
		Description: "Hoody",
		Price:       8000,
		Currency:    "EUR",
		Quantity:    1,
	},
	{
		UID:         "product_tennis_racket",
		Description: "Tennis racket",
		Price:       16900,
		Currency:    "EUR",
		Quantity:    1,
	},
	{
		UID:         "product_tennis_balls",
		Description: "Tennis balls",
		Price:       1000,
		Currency:    "EUR",
		Quantity:    6,
	},
	{
		UID:         "product_tennis_shoes",
		Description: "Tennis shoes",
		Price:       12000,
		Currency:    "EUR",
		Quantity:    1,
	},
	{
		UID:         "product_running_shoes",
		Description: "Running shoes",
		Price:       12000,
		Currency:    "EUR",
		Quantity:    1,
	},
	{
		UID:         "product_running_shirt",
		Description: "Running shirt",
		Price:       5000,
		Currency:    "EUR",
		Quantity:    1,
	},
	{
		UID:         "product_running_shorts",
		Description: "Running shorts",
		Price:       4000,
		Currency:    "EUR",
		Quantity:    1,
	},
	{

		UID:         "product_running_socks",
		Description: "Running socks",
		Price:       1000,
		Currency:    "EUR",
		Quantity:    3,
	},
	{
		UID:         "product_running_cap",
		Description: "Running cap",
		Price:       2000,
		Currency:    "EUR",
		Quantity:    1,
	},
}
