package dto

type SuccessRegisterDTO struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

type SuccessLoginedDto struct {
	Status       int    `json:"status"`
	Message      string `json:"message"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
}
