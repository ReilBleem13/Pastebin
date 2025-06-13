package dto

type RequestCreatePasta struct {
	Message    string  `json:"message"`
	Language   *string `json:"language"`
	Expiration *string `json:"expiration"`
	Visibility *string `json:"visibility"`
	Password   *string `json:"password"`
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
