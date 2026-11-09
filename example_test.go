package validator_test

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/getrapid-app/go-validator"
)

// Validating a map in a plain program, such as a CLI or a queue worker.
func ExampleValidate() {
	input := map[string]any{"email": "not-an-email", "age": 15}
	err := validator.Validate(context.Background(), input, validator.Rules{
		"email": []string{"required", "email"},
		"age":   []string{"required", "integer", "min:18"},
	})
	if bag, ok := validator.ErrorsOf(err); ok {
		for _, field := range bag.Fields() {
			fmt.Println(bag.First(field))
		}
	}
	// Output:
	// The age field must be at least 18.
	// The email field must be a valid email address.
}

type Signup struct {
	Email    string `json:"email" validate:"required|email"`
	Password string `json:"password" validate:"required|min:8"`
}

func (s *Signup) Rules() validator.Rules {
	return validator.Rules{"password_confirmation": []string{"required", "same:password"}}
}

// Decoding and validating a request type in one call, from JSON a job or a
// message queue delivered.
func ExampleBind() {
	var input map[string]any
	_ = json.Unmarshal([]byte(`{"email":"jane@example.com","password":"secret123","password_confirmation":"secret"}`), &input)

	req, err := validator.Bind[Signup](context.Background(), input)
	out, _ := json.Marshal(err.(*validator.ValidationError).Errors)
	fmt.Println(req.Email)
	fmt.Println(string(out))
	// Output:
	// jane@example.com
	// {"password_confirmation":["The password confirmation field must match password."]}
}

// Defining a custom rule once and using it in every syntax.
func ExampleDefine() {
	reg := validator.NewRegistry()
	reg.Define("even").
		Message("The :attribute must be even.").
		Check(func(_ context.Context, f validator.Field, _ []string) error {
			if n, ok := validator.As[int](f.Value); !ok || n%2 != 0 {
				return validator.Fail("even", nil)
			}
			return nil
		})
	v := validator.New(validator.WithRegistry(reg))

	err := v.Validate(context.Background(), map[string]any{"seats": 3}, validator.Rules{
		"seats": []string{"required", "integer", "even"},
	})
	fmt.Println(err)
	// Output: The seats must be even.
}

func ExampleTranslations() {
	// Usually validator.AddTranslations, which changes the defaults
	// everywhere. A separate catalog keeps this example self-contained.
	tr := validator.NewTranslations().Add("ar-EG", map[string]string{
		"required":         "لازم تكتب :attribute.",
		"attributes.email": "الإيميل",
	})
	v := validator.New(validator.WithTranslator(tr, "en"))

	rules := validator.Rules{"email": "required", "age": "required|integer"}
	err := v.Validate(validator.ContextWithLocale(context.Background(), "ar-EG"), map[string]any{"age": "x"}, rules)
	bag, _ := validator.ErrorsOf(err)
	fmt.Println(bag.First("email"))
	fmt.Println(bag.First("age")) // not overridden: built-in Arabic
	// Output:
	// لازم تكتب الإيميل.
	// يجب أن يكون حقل age عددًا صحيحًا.
}
