package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/devanshu0x/chirpy-backend/internal/auth"
	"github.com/devanshu0x/chirpy-backend/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileServerHits atomic.Int32
	dbQueries      *database.Queries
	secret         string
	polka_key string
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) reset() {
	cfg.fileServerHits.Store(0)
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(code)
	w.Write(encoded)
	return nil
}

func respondWithError(w http.ResponseWriter, code int, msg string) error {
	return respondWithJSON(w, code, map[string]string{"error": msg})
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to db: %v", err)
	}

	mux := http.NewServeMux()
	apiCfg := apiConfig{dbQueries: database.New(db), secret: os.Getenv("SECRET_KEY"), polka_key: os.Getenv("POLKA_KEY")}
	mux.Handle("/app/", http.StripPrefix("/app", apiCfg.middlewareMetricsInc(http.FileServer(http.Dir(".")))))
	server := http.Server{
		Handler: mux,
		Addr:    ":8080",
	}
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("content-type", "text/plain; charset=utf-8")
		w.WriteHeader(200)

		w.Write([]byte("OK"))
	})

	mux.HandleFunc("GET /admin/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("content-type", "text/html")
		responseString := fmt.Sprintf("<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p> </body></html>", apiCfg.fileServerHits.Load())
		w.Write([]byte(responseString))
	})

	mux.HandleFunc("POST /admin/reset", func(w http.ResponseWriter, r *http.Request) {
		// apiCfg.reset()
		err := apiCfg.dbQueries.DeleteAllUsers(r.Context())
		if err != nil {
			fmt.Printf("Error while deleting users: %v", err)
			respondWithError(w, 500, "Failed to delete users")
			return 
		}
		w.Header().Set("content-type", "text/plain")
		w.Write([]byte("All users deleted!"))
	})

	mux.HandleFunc("POST /api/chirps", func(w http.ResponseWriter, r *http.Request) {
		token,err:=auth.GetBearerToken(r.Header)
		if err!=nil{
			respondWithError(w,401,"Failed to get token in req")
			return
		}

		userId,err:=auth.ValidateJWT(token,apiCfg.secret)
		if err!=nil{
			respondWithError(w,401,"You are unauthorized")
			return
		}

		type params struct {
			Body   string    `json:"body"`
			UserId uuid.UUID `json:"user_id"`
		}
		defer r.Body.Close()
		param := params{}
		decoder := json.NewDecoder(r.Body)
		err = decoder.Decode(&param)
		if err != nil {
			if resErr := respondWithError(w, 501, "something went wrong"); resErr != nil {
				fmt.Printf("failed to write response: %v", resErr)
			}
		}
		param.UserId=userId
		if len(param.Body) > 140 {
			if resErr := respondWithError(w, 400, "Chirp is too long"); resErr != nil {
				fmt.Printf("failed to write response: %v", resErr)
			}
		} else {

			words := strings.Split(param.Body, " ")
			for ind, word := range words {
				lowerWord := strings.ToLower(word)
				if lowerWord == "kerfuffle" || lowerWord == "sharbert" || lowerWord == "fornax" {
					words[ind] = "****"
				}
			}

			result := strings.Join(words, " ")
			arg := database.CreateChirpParams{
				Body:   result,
				UserID: param.UserId,
			}
			dbChirp, err := apiCfg.dbQueries.CreateChirp(r.Context(), arg)
			if err != nil {
				fmt.Printf("Failed to save chirp: %v", err)
				respondWithError(w, 500, "Failed to save chirp")
				return
			}

			chirp := Chirp{
				ID:        dbChirp.ID,
				CreatedAt: dbChirp.CreatedAt,
				UpdatedAt: dbChirp.UpdatedAt,
				Body:      dbChirp.Body,
				UserID:    dbChirp.UserID,
			}

			if resErr := respondWithJSON(w, 201, chirp); resErr != nil {
				fmt.Printf("failed to write response: %v", resErr)
			}
		}

	})

	mux.HandleFunc("POST /api/users", func(w http.ResponseWriter, r *http.Request) {
		body := &struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}{}
		defer r.Body.Close()

		

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(body); err != nil {
			fmt.Printf("Failed to decode body: %v", err)
			respondWithError(w, 501, "Failed to decode body")
			return
		}

		hashedPass, err := auth.HashPassword(body.Password)
		if err != nil {
			respondWithError(w, 501, "Failed to hash password")
			return
		}

		params := database.CreateUserParams{
			Email:          body.Email,
			HashedPassword: hashedPass,
		}
		dbUser, err := apiCfg.dbQueries.CreateUser(r.Context(), params)
		if err != nil {
			fmt.Printf("Failed to create user: %v", err)
			respondWithError(w, 501, "Failed to create user")
			return
		}

		user := User{
			ID:        dbUser.ID,
			CreatedAt: dbUser.CreatedAt,
			UpdatedAt: dbUser.UpdatedAt,
			Email:     dbUser.Email,
			IsChirpyRed: dbUser.IsChirpyRed.Bool,
		}

		respondWithJSON(w, 201, user)
	})

	mux.HandleFunc("GET /api/chirps", func(w http.ResponseWriter, r *http.Request) {
		authorId:=r.URL.Query().Get("author_id")
		var dbChirps []database.Chirp
		if authorId!=""{
			id,err:=uuid.Parse(authorId)
			if err!=nil{
				w.WriteHeader(404)
				return
			}
			dbChirps,err = apiCfg.dbQueries.GetChirpOfAuthor(r.Context(),id)
		}else{
			dbChirps, err = apiCfg.dbQueries.GetAllChirps(r.Context())
		}
		if err != nil {
			fmt.Printf("Failed to fetch chirps: %v", err)
			respondWithError(w, 500, "Failed to fetch chirps")
			return
		}
		chirps := make([]Chirp, len(dbChirps))

		for i, dbChirp := range dbChirps {
			chirps[i] = Chirp{
				ID:        dbChirp.ID,
				CreatedAt: dbChirp.CreatedAt,
				UpdatedAt: dbChirp.UpdatedAt,
				Body:      dbChirp.Body,
				UserID:    dbChirp.UserID,
			}
		}

		sortOrder:=r.URL.Query().Get("sort")

		if sortOrder=="desc"{
			sort.Slice(chirps,func(i, j int) bool {
				return chirps[i].CreatedAt.After(chirps[j].CreatedAt)
			})
		}

		respondWithJSON(w, 200, chirps)
	})

	mux.HandleFunc("GET /api/chirps/{chirpId}", func(w http.ResponseWriter, r *http.Request) {
		chirpId := r.PathValue("chirpId")
		id, err := uuid.Parse(chirpId)
		if err != nil {
			fmt.Printf("Unable to parse chirp id as uuid")
			respondWithError(w, 404, "Failed to parse uuid")
			return

		}
		dbChirp, err := apiCfg.dbQueries.GetChrip(r.Context(), id)
		if err != nil {
			respondWithError(w, 404, "No chirp found")
			return
		}
		chirp := Chirp{
			ID:        dbChirp.ID,
			CreatedAt: dbChirp.CreatedAt,
			UpdatedAt: dbChirp.UpdatedAt,
			Body:      dbChirp.Body,
			UserID:    dbChirp.UserID,
		}
		respondWithJSON(w, 200, chirp)
	})

	mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		body := &struct {
			Email            string `json:"email"`
			Password         string `json:"password"`
			ExpiresInSeconds int    `json:"expires_in_seconds"`
		}{}
		defer r.Body.Close()

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(body); err != nil {
			fmt.Printf("Failed to decode body: %v", err)
			respondWithError(w, 501, "Failed to decode body")
			return
		}

		dbUser, err := apiCfg.dbQueries.FindUser(r.Context(), body.Email)
		if err != nil {
			respondWithError(w, 404, "User not found")
			return
		}

		validPass, err := auth.CheckPasswordHash(body.Password, dbUser.HashedPassword)
		if err != nil {
			respondWithError(w, 500, "Error occured while verifying password")
			return
		}

		if !validPass {
			respondWithError(w, 401, "incorrect email or password")
			return
		}
		expiresIn := time.Duration(body.ExpiresInSeconds) * time.Second
		if expiresIn == time.Duration(0) {
			expiresIn = time.Hour
		}
		jwt, err := auth.MakeJWT(dbUser.ID, apiCfg.secret, expiresIn)
		if err != nil {
			respondWithError(w, 500, "Failed to create JWT")
			return
		}

		refreshToken:=auth.MakeRefereshToken()

		err=apiCfg.dbQueries.AddRefreshToken(r.Context(),database.AddRefreshTokenParams{
			Token: refreshToken,
			UserID: dbUser.ID,
		})
		if err!=nil{
			respondWithError(w,500,"Failed to generate refresh token")
			return
		}

		user := LoginResponse{
			User: User{
				ID:        dbUser.ID,
				CreatedAt: dbUser.CreatedAt,
				UpdatedAt: dbUser.UpdatedAt,
				Email:     dbUser.Email,
				IsChirpyRed: dbUser.IsChirpyRed.Bool,
			},
			Token: jwt,
			RefreshToken: refreshToken,
		}

		respondWithJSON(w, 200, user)

	})


	mux.HandleFunc("POST /api/refresh",func(w http.ResponseWriter, r *http.Request) {
		token,err:=auth.GetBearerToken(r.Header)
		if err!=nil{
			respondWithError(w,401,"Token not found")
			return
		}

		refToken,err:=apiCfg.dbQueries.ValidateRefreshToken(r.Context(),token)
		if err!=nil{
			respondWithError(w,401,"Token not found in db")
			return
		}

		if refToken.ExpiresAt.Before(time.Now()) || refToken.RevokedAt.Valid{
			respondWithError(w,401,"Token expired or revoked")
			return
		}

		accessToken,err:=auth.MakeJWT(refToken.UserID,apiCfg.secret,time.Hour)
		if err!=nil{
			respondWithError(w,401,"Failed to make jwt")
			return
		}

		respondWithJSON(w,200,map[string]string{
			"token": accessToken,
		})
	})

	mux.HandleFunc("POST /api/revoke",func(w http.ResponseWriter, r *http.Request) {
		token,err:=auth.GetBearerToken(r.Header)
		if err!=nil{
			respondWithError(w,401,"Token not found")
			return
		}

		err=apiCfg.dbQueries.RevokeRefreshToken(r.Context(),token)
		if err!=nil{
			respondWithError(w,401,"Token not found in db")
			return
		}
		
		w.WriteHeader(204)
	})

	mux.HandleFunc("PUT /api/users",func(w http.ResponseWriter, r *http.Request) {
		body:=&struct{
			Email string `json:"email"`
			Password string `json:"password"`
		}{}

		defer r.Body.Close()

		decoder:=json.NewDecoder(r.Body)
		if err:= decoder.Decode(body);err!=nil{
			respondWithError(w,401,"Failed to decode body")
			return
		}

		token,err:=auth.GetBearerToken(r.Header)
		if err!=nil{
			respondWithError(w,401,"Failed to get token")
			return
		}

		userId,err:=auth.ValidateJWT(token,apiCfg.secret)
		if err!=nil{
			respondWithError(w,401,"Wrong token")
			return
		}
		
		hashedPass,err:=auth.HashPassword(body.Password)
		if err!=nil{
			respondWithError(w,500,"Failed to hash password")
			return
		}

		dbUser,err:=apiCfg.dbQueries.UpdateUser(r.Context(),database.UpdateUserParams{
			Email: body.Email,
			HashedPassword: hashedPass,
			ID: userId,
		})

		if err!=nil{
			respondWithError(w,401,"Failed to update db")
			return
		}

		user:=User{
			ID: dbUser.ID,
			CreatedAt: dbUser.CreatedAt,
			UpdatedAt: dbUser.UpdatedAt,
			Email: dbUser.Email,
			IsChirpyRed: dbUser.IsChirpyRed.Bool,
		}

		respondWithJSON(w,200,user)
	})

	mux.HandleFunc("DELETE /api/chirps/{chirpID}",func(w http.ResponseWriter, r *http.Request) {
		chirpId := r.PathValue("chirpID")
		id, err := uuid.Parse(chirpId)
		if err != nil {
			fmt.Printf("Unable to parse chirp id as uuid")
			respondWithError(w, 404, "Failed to parse uuid")
			return

		}
		dbChirp, err := apiCfg.dbQueries.GetChrip(r.Context(), id)
		if err != nil {
			respondWithError(w, 404, "No chirp found")
			return
		}

		token,err:=auth.GetBearerToken(r.Header)
		if err!=nil{
			respondWithError(w,401,"Failed to get token")
			return
		}

		userId,err:=auth.ValidateJWT(token,apiCfg.secret)
		if err!=nil{
			respondWithError(w,401,"Wrong token")
			return
		}

		if dbChirp.UserID!=userId{
			respondWithError(w,403,"You are not you!")
			return
		}

		err=apiCfg.dbQueries.DeleteChirp(r.Context(),dbChirp.ID)
		if err!=nil{
			respondWithError(w,500,"Failed to delete chirp")
			return
		}

		respondWithJSON(w,204,map[string]string{
			"message":"Deleted",
		})
		
	})

	mux.HandleFunc("POST /api/polka/webhooks",func(w http.ResponseWriter, r *http.Request) {
		body:=&struct{
			Event string `json:"event"`
			Data struct{
				UserID uuid.UUID `json:"user_id"`
			} `json:"data"`
		}{}
		
		api,err:=auth.GetAPIKey(r.Header)
		if err!=nil{
			respondWithError(w,401,"Failed to parse api key")
			return
		}

		if api!=apiCfg.polka_key{
			w.WriteHeader(401)
			return
		}

		defer r.Body.Close()		
		decoder:=json.NewDecoder(r.Body)
		if err:=decoder.Decode(body);err!=nil{
			respondWithError(w,500,"Failed to parse json body")
			return
		}

		if body.Event!="user.upgraded"{
			w.WriteHeader(204)
			return
		}

		_,err=apiCfg.dbQueries.MakeUserChirpyRed(r.Context(),body.Data.UserID)
		if err!=nil{
			w.WriteHeader(404)
			return
		}

		w.WriteHeader(204)
	})

	serverErr := server.ListenAndServe()
	if serverErr != nil {
		fmt.Printf("Error in starting sever: %v", serverErr)
	}

}
