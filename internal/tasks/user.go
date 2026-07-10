package tasks

import "errors"

var ErrUserNotFound = errors.New("user not found")
var ErrUserAlreadyExists = errors.New("user already exists")
var ErrInvalidCredentials = errors.New("invalid username or password")

type User struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
}

type RegisterRequest struct {
	Username   string `json:"username" validate:"required,min=2,max=50"`
	Password   string `json:"password" validate:"required,min=6,max=50"`
	InviteCode string `json:"invite_code" validate:"required"`
}

// LoginRequest — DTO для контракта входа в систему.
type LoginRequest struct {
	Username string `json:"username" validate:"required,min=2,max=50"`
	Password string `json:"password" validate:"required"` // При логине длину min=6 проверять не обязательно, это забота базы
}
