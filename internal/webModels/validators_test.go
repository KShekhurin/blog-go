package webModels

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoginValidation(t *testing.T) {
	v := validator.New()

	v.RegisterStructValidation(loginValidation, UserLoginInfo{})

	tests := []struct {
		name         string
		input        UserLoginInfo
		wantFields   []string
		wantNoErrors bool
	}{
		{
			name:         "login only",
			input:        UserLoginInfo{Login: "tom", Password: "password"},
			wantNoErrors: true,
		},
		{
			name:         "email only",
			input:        UserLoginInfo{Email: "tom@gmail.com", Password: "password"},
			wantNoErrors: true,
		},
		{
			name:         "email and login",
			input:        UserLoginInfo{Login: "tom", Email: "tom@gmail.com", Password: "password"},
			wantNoErrors: true,
		},
		{
			name:  "no email, no login",
			input: UserLoginInfo{Password: "password"},
			wantFields: []string{
				"login",
				"email",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Struct(tt.input)

			if tt.wantNoErrors {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)

			validationErrs, ok := err.(validator.ValidationErrors)
			require.True(t, ok)

			gotFields := make([]string, 0, len(validationErrs))
			for _, e := range validationErrs {
				gotFields = append(gotFields, e.StructField())
			}

			assert.ElementsMatch(t, tt.wantFields, gotFields)
		})
	}
}
