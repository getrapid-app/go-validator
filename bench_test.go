package validator_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/getrapid-app/go-validator"
)

type benchItem struct {
	SKU   string  `json:"sku" validate:"required|string|max:32"`
	Qty   int     `json:"qty" validate:"required|integer|min:1"`
	Price float64 `json:"price" validate:"required|numeric|min:0"`
}

type benchOrder struct {
	Email   string      `json:"email" validate:"required|email|max:255"`
	Name    string      `json:"name" validate:"required|string|between:2,100"`
	Country string      `json:"country" validate:"required|in:EG,US,GB,DE"`
	Items   []benchItem `json:"items" validate:"required|array|min:1"`
}

func benchItems(n int) []benchItem {
	items := make([]benchItem, n)
	for i := range items {
		items[i] = benchItem{SKU: fmt.Sprintf("sku-%d", i), Qty: 1 + i%5, Price: 9.99}
	}
	return items
}

func BenchmarkStructTags(b *testing.B) {
	o := benchOrder{Email: "jane@example.com", Name: "Jane", Country: "EG", Items: benchItems(3)}
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		if err := validator.Struct(ctx, &o); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMapRules(b *testing.B) {
	in := map[string]any{"email": "jane@example.com", "name": "Jane", "age": 30.0, "country": "EG"}
	rules := validator.Rules{
		"email":   []string{"required", "email", "max:255"},
		"name":    []string{"required", "string", "between:2,100"},
		"age":     []string{"required", "integer", "min:18"},
		"country": []string{"required", "in:EG,US,GB,DE"},
	}
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		if err := validator.Validate(ctx, in, rules); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWildcard100Items(b *testing.B) {
	o := benchOrder{Email: "jane@example.com", Name: "Jane", Country: "EG", Items: benchItems(100)}
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		if err := validator.Struct(ctx, &o); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFailingWithMessages(b *testing.B) {
	o := benchOrder{Email: "nope", Country: "FR", Items: []benchItem{{}}}
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		if err := validator.Struct(ctx, &o); err == nil {
			b.Fatal("want errors")
		}
	}
}
