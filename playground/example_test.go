package playground_test

import (
	"context"
	"fmt"

	"github.com/getrapid-app/go-validator"
	"github.com/getrapid-app/go-validator/playground"
)

func Example() {
	playground.Use() // once, at program start

	type Signup struct {
		Phone   string `json:"phone" validate:"required|pg.e164"`
		Country string `json:"country" validate:"required|pg.iso3166_1_alpha2"`
		Color   string `json:"color" validate:"pg.oneof:red,green,blue"`
	}
	err := validator.Struct(context.Background(), &Signup{Phone: "0100", Country: "EG", Color: "pink"})
	bag, _ := validator.ErrorsOf(err)
	for _, field := range bag.Fields() {
		fmt.Println(bag.First(field))
	}
	// Output:
	// The phone field must be a phone number in E.164 format.
	// The color field must be one of: red, green, blue.
}
