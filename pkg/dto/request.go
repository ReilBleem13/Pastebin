package dto

type RequestCreatePasta struct {
	Message    string `json:"message" binding:"required"`
	Language   string `json:"language"`
	Expiration string `json:"expiration"`
	Visibility string `json:"visibility"`
	Password   string `json:"password"`
}

type RequestNewUser struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginUser struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type Password struct {
	Password string `json:"password"`
}

// в api слое проверить корректность данных, а в сервисе соответствие правилам
