package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func HashPassword(password string) (string,error){
	return argon2id.CreateHash(password,argon2id.DefaultParams)
}

func CheckPasswordHash(password, hash string) (bool,error){
	check,_,err:=argon2id.CheckHash(password,hash)
	return check,err
}

func MakeJWT(userId uuid.UUID, tokenSecret string, expiresIn time.Duration) (string,error){
	token:=jwt.NewWithClaims(jwt.SigningMethodHS256,jwt.RegisteredClaims{
		Issuer: "chirpy-access",
		Subject: userId.String(),
		IssuedAt: jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
	})
	return token.SignedString([]byte(tokenSecret))
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID,error){
	token,err:=jwt.ParseWithClaims(tokenString,&jwt.RegisteredClaims{},func(t *jwt.Token) (any, error) {
		if _,ok:= t.Method.(*jwt.SigningMethodHMAC);!ok{
			return nil,fmt.Errorf("Unexpected signing method")
		}
		return []byte(tokenSecret),nil
	})
	if err!=nil{
		return uuid.Nil,err
	}

	claims,ok:= token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid{
		return uuid.Nil, fmt.Errorf("invalid token")
	}

	userId,err:= uuid.Parse(claims.Subject)
	if err!=nil{
		return uuid.Nil,err
	}

	return userId,nil

}

func GetBearerToken(headers http.Header) (string,error){
	authorization:=headers.Get("authorization")
	if authorization==""{
		return "",fmt.Errorf("No authorization header present")
	}

	vals:=strings.Split(authorization," ")
	if len(vals)==2 && vals[0]=="Bearer"{
		return vals[1],nil
	}
	
	return "",fmt.Errorf("Authorization not in valid format")
	
}

func GetAPIKey(headers http.Header) (string,error){
	authorization:=headers.Get("authorization")
	if authorization==""{
		return "",fmt.Errorf("No authorization header present")
	}

	vals:=strings.Split(authorization," ")
	if len(vals)==2 && vals[0]=="ApiKey"{
		return vals[1],nil
	}
	
	return "",fmt.Errorf("Authorization not in valid format")
	
}


func MakeRefereshToken() string{
	token:=make([]byte, 32)
	rand.Read(token)
	return hex.EncodeToString(token)
}