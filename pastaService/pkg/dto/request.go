package dto

type RequestCreatePasta struct {
	Message    string `json:"message" binding:"required"`
	Language   string `json:"language"`
	Expiration string `json:"expiration"`
	Visibility string `json:"visibility"`
	Password   string `json:"password"`
}

type Password struct {
	Password string `json:"password"`
}
