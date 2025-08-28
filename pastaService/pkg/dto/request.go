package dto

type RequestCreatePasta struct {
	Message    string `json:"message" binding:"required"`
	Language   string `json:"language"`
	Expiration string `json:"expiration"`
	Visibility string `json:"visibility"`
	Password   string `json:"password"`

	ExpireAfterRead bool `json:"expire_after_read"`
}

type UpdateRequest struct {
	NewText  string `json:"new_text" binding:"required"`
	Password string `json:"password"`
}

type Password struct {
	Password string `json:"password"`
}
