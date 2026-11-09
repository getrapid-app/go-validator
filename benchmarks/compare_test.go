// Package benchmarks compares go-validator with go-playground/validator.
// It is a separate module so the main module keeps no dependencies.
//
//	cd benchmarks && go test -bench . -benchmem
package benchmarks

import (
	"context"
	"fmt"
	"testing"

	playground "github.com/go-playground/validator/v10"

	"github.com/getrapid-app/go-validator"
)

// The same order, written for both libraries.

type item struct {
	SKU   string  `json:"sku" validate:"required|string|max:32" pg:"required,max=32"`
	Qty   int     `json:"qty" validate:"required|integer|min:1" pg:"required,min=1"`
	Price float64 `json:"price" validate:"required|numeric|min:0" pg:"required,min=0"`
}

type order struct {
	Email   string `json:"email" validate:"required|email|max:255" pg:"required,email,max=255"`
	Name    string `json:"name" validate:"required|string|between:2,100" pg:"required,min=2,max=100"`
	Country string `json:"country" validate:"required|in:EG,US,GB,DE" pg:"required,oneof=EG US GB DE"`
	Items   []item `json:"items" validate:"required|array|min:1" pg:"required,min=1,dive"`
}

func items(n int) []item {
	out := make([]item, n)
	for i := range out {
		out[i] = item{SKU: fmt.Sprintf("sku-%d", i), Qty: 1 + i%5, Price: 9.99}
	}
	return out
}

var pg = func() *playground.Validate {
	v := playground.New(playground.WithRequiredStructEnabled())
	v.SetTagName("pg")
	return v
}()

var ctx = context.Background()

func valid(n int) order {
	return order{Email: "jane@example.com", Name: "Jane", Country: "EG", Items: items(n)}
}

func invalid() order {
	return order{Email: "nope", Country: "FR", Items: []item{{}}}
}

func TestBothAgree(t *testing.T) {
	o := valid(3)
	if err := validator.Struct(ctx, &o); err != nil {
		t.Fatal(err)
	}
	if err := pg.Struct(&o); err != nil {
		t.Fatal(err)
	}
	bad := invalid()
	bag, _ := validator.ErrorsOf(validator.Struct(ctx, &bad))
	pgErrs := pg.Struct(&bad).(playground.ValidationErrors)
	if bag.Len() != len(pgErrs) {
		t.Fatalf("fields: ours %v, playground %v", bag.Fields(), pgErrs)
	}
}

func BenchmarkStructValid(b *testing.B) {
	o := valid(3)
	b.Run("go-validator", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := validator.Struct(ctx, &o); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("playground", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := pg.Struct(&o); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkStruct100Items(b *testing.B) {
	o := valid(100)
	b.Run("go-validator", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := validator.Struct(ctx, &o); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("playground", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := pg.Struct(&o); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkStructInvalid includes turning every failure into a message,
// since that is what an API returns.
func BenchmarkStructInvalid(b *testing.B) {
	o := invalid()
	b.Run("go-validator", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			bag, ok := validator.ErrorsOf(validator.Struct(ctx, &o))
			if !ok || bag.Len() == 0 {
				b.Fatal("want errors")
			}
		}
	})
	b.Run("playground", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			errs, ok := pg.Struct(&o).(playground.ValidationErrors)
			if !ok || len(errs) == 0 {
				b.Fatal("want errors")
			}
			msgs := make(map[string]string, len(errs))
			for _, e := range errs {
				msgs[e.Namespace()] = e.Error()
			}
		}
	})
}

func BenchmarkMap(b *testing.B) {
	in := map[string]any{"email": "jane@example.com", "name": "Jane", "age": 30, "country": "EG"}
	ours := validator.Rules{
		"email":   "required|email|max:255",
		"name":    "required|string|between:2,100",
		"age":     "required|integer|min:18",
		"country": "required|in:EG,US,GB,DE",
	}
	theirs := map[string]any{
		"email":   "required,email,max=255",
		"name":    "required,min=2,max=100",
		"age":     "required,min=18",
		"country": "required,oneof=EG US GB DE",
	}
	b.Run("go-validator", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := validator.Validate(ctx, in, ours); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("playground", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if errs := pg.ValidateMap(in, theirs); len(errs) != 0 {
				b.Fatal(errs)
			}
		}
	})
}
