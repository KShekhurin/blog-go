package webModels

import (
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func RegisterValidators() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterStructValidation(loginValidation, UserLoginInfo{})
	}
}

func loginValidation(sl validator.StructLevel) {
	//If neither email nor login was provided input is invalid

	loginInfo := sl.Current().Interface().(UserLoginInfo)

	if loginInfo.Email == "" && loginInfo.Login == "" {
		sl.ReportError(loginInfo.Login, "Login", "login", "required_if_no_email", "")
		sl.ReportError(loginInfo.Password, "Email", "email", "required_if_no_login", "")
	}
}
