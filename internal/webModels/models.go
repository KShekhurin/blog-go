package webModels

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type UserRegisterInfo struct {
	Login    string `json:"login" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type UserLoginInfo struct {
	Login    string `json:"login"`
	Email    string `json:"email"`
	Password string `json:"password"`
}
