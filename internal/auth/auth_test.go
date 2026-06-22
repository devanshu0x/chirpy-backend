package auth

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMakeAndValidateJWT(t *testing.T) {
	userID := uuid.New()
	secret := "super-secret"

	token, err := MakeJWT(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("MakeJWT returned error: %v", err)
	}

	gotUserID, err := ValidateJWT(token, secret)
	if err != nil {
		t.Fatalf("ValidateJWT returned error: %v", err)
	}

	if gotUserID != userID {
		t.Errorf("expected %v, got %v", userID, gotUserID)
	}
}

func TestValidateJWTExpired(t *testing.T) {
	userID := uuid.New()
	secret := "super-secret"

	token, err := MakeJWT(userID, secret, -time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ValidateJWT(token, secret)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateJWTWrongSecret(t *testing.T) {
	userID := uuid.New()

	token, err := MakeJWT(userID, "secret1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ValidateJWT(token, "secret2")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestHash(t *testing.T){
	password:="124325fhdsjkgh"

	hash,err:=HashPassword(password)
	if err!=nil{
		t.Fatal("Failed to hash password")
	}

	ok,err:=CheckPasswordHash(password,hash)
	if err!=nil{
		t.Fatal("Failed to verify hash")
	}

	if !ok{
		t.Fatal("Failed to verify hash with password")
	}
}

func TestInvalidHash(t *testing.T){
	password:="124325fhdsjkgh"

	hash,err:=HashPassword(password)
	if err!=nil{
		t.Fatal("Failed to hash password")
	}

	ok,err:=CheckPasswordHash("1234",hash)
	if err!=nil{
		t.Fatal("Failed to verify hash")
	}

	if ok{
		t.Fatal("Wrong password validated agaisnt hash")
	}
}

func TestGetBearerToken(t *testing.T){
	header1:=http.Header{}
	header1.Add("authorization","Bearer halwa")

	header2:=http.Header{}
	header2.Set("authorization","Bearer")

	header3:=http.Header{}

cases:=[]struct{
	input http.Header
	valid bool
	output string
}{
	{
		input: header1,
		valid:true,
		output: "halwa",
	},
	{
		input: header2,
		valid:false,
		output: "",
	},
	{
		input: header3,
		valid:false,
		output: "",
	},
}

for _,entry:= range cases{
	token,err:=GetBearerToken(entry.input)
	if entry.valid{
		if err!=nil{
			t.Errorf("Failed to get token")
		}
		if token!=entry.output{
			t.Errorf("Failed to get correct valid token")
		}
	}else{
		if err==nil{
			t.Error("Invalid token is getting parsed")
		}
	}
}

}