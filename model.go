package main

import (
	"time"
	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time	`json:"updated_at"`
	Email     string	`json:"email"`
	IsChirpyRed bool `json:"is_chirpy_red"`
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string	`json:"body"`
	UserID    uuid.UUID  `json:"user_id"`
}

type LoginResponse struct{
	User
	Token string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}
type RefreshToken struct{
	Token string `json:"token"`
	UserId uuid.UUID `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at"`
}