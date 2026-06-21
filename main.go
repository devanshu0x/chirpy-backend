package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/devanshu0x/chirpy-backend/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct{
	fileServerHits atomic.Int32
	dbQueries *database.Queries
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler{
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){
		cfg.fileServerHits.Add(1)
		next.ServeHTTP(w,r)
	})
}

func (cfg *apiConfig) reset() {
	cfg.fileServerHits.Store(0)
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) error {
	encoded,err:= json.Marshal(payload)
	if err!=nil{
		return err
	}
	w.Header().Set("content-type","application/json")
	w.WriteHeader(code)
	w.Write(encoded)
	return nil
}

func respondWithError(w http.ResponseWriter,code int, msg string) error {
	return respondWithJSON(w,code, map[string] string{"error":msg})
}

func  main(){
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err!=nil{
		log.Fatalf("Failed to connect to db: %v",err)
	}

	mux:= http.NewServeMux()
	apiCfg:=apiConfig{dbQueries: database.New(db)}
	mux.Handle("/app/",http.StripPrefix("/app",apiCfg.middlewareMetricsInc(http.FileServer(http.Dir(".")))))
	server:=http.Server{
		Handler: mux,
		Addr: ":8080",
	}
	mux.HandleFunc("GET /api/healthz", func (w http.ResponseWriter, req *http.Request){
		w.Header().Add("content-type","text/plain; charset=utf-8")
		w.WriteHeader(200)

		w.Write([]byte("OK"))
	})

	mux.HandleFunc("GET /admin/metrics",func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("content-type","text/html")
		responseString:=fmt.Sprintf("<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p> </body></html>",apiCfg.fileServerHits.Load())
		w.Write([]byte(responseString))
	})
	
	mux.HandleFunc("POST /admin/reset", func(w http.ResponseWriter, r *http.Request) {
		apiCfg.reset()
		err:=apiCfg.dbQueries.DeleteAllUsers(r.Context())
		if err!=nil{
			fmt.Printf("Error while deleting users: %v",err)
			respondWithError(w,500,"Failed to delete users")
		}
		w.Header().Set("content-type","text/plain")
		w.Write([]byte("All users deleted!"))
	})

	mux.HandleFunc("POST /api/chirps",func(w http.ResponseWriter, r *http.Request) {
		type params struct{
			Body string `json:"body"`
			UserId uuid.UUID `json:"user_id"`
		}
		defer r.Body.Close()
		param:=params{}
		decoder:=json.NewDecoder(r.Body)
		err:=decoder.Decode(&param)
		if err!=nil{
			if resErr:=respondWithError(w,501,"something went wrong");resErr!=nil{
				fmt.Printf("failed to write response: %v",resErr)
			}
		}
		if len(param.Body)>140{
			if resErr:=respondWithError(w,400,"Chirp is too long");resErr!=nil{
				fmt.Printf("failed to write response: %v",resErr)
			}
		}else{

			words:=strings.Split(param.Body," ")
			for ind,word:= range words{
				lowerWord:=strings.ToLower(word)
				if(lowerWord=="kerfuffle" || lowerWord=="sharbert" || lowerWord=="fornax"){
					words[ind]="****"
				}
			}

			result:=strings.Join(words," ")
			arg:=database.CreateChirpParams{
				Body: result,
				UserID: param.UserId,
			}
			dbChirp,err:= apiCfg.dbQueries.CreateChirp(r.Context(),arg)
			if err!=nil{
				fmt.Printf("Failed to save chirp: %v",err)
				respondWithError(w,500,"Failed to save chirp")
			}

			chirp:=Chirp{
				ID: dbChirp.ID,
				CreatedAt: dbChirp.CreatedAt,
				UpdatedAt: dbChirp.UpdatedAt,
				Body: dbChirp.Body,
				UserID: dbChirp.UserID,
			}

			if resErr:=respondWithJSON(w,201,chirp);resErr!=nil{
				fmt.Printf("failed to write response: %v",resErr)
			}
		}

	})

	mux.HandleFunc("POST /api/users", func(w http.ResponseWriter, r *http.Request) {
		body:= &struct{
			Email string `json:"email"`
		}{}
		defer r.Body.Close()

		decoder:=json.NewDecoder(r.Body)
		if err:=decoder.Decode(body);err!=nil{
			fmt.Printf("Failed to decode body: %v",err)
			respondWithError(w,501,"Failed to decode body")
			return
		}

		dbUser,err:=apiCfg.dbQueries.CreateUser(r.Context(),body.Email)
		if err!=nil{
			fmt.Printf("Failed to create user: %v",err)
			respondWithError(w,501,"Failed to create user")
			return
		}

		user:=User{
			ID: dbUser.ID,
			CreatedAt: dbUser.CreatedAt,
			UpdatedAt: dbUser.UpdatedAt,
			Email: dbUser.Email,
		}

		respondWithJSON(w,201,user)
	})

	mux.HandleFunc("GET /api/chirps",func(w http.ResponseWriter, r *http.Request) {
		dbChirps,err:=apiCfg.dbQueries.GetAllChirps(r.Context())
		if err!=nil{
			fmt.Printf("Failed to fetch chirps: %v",err)
			respondWithError(w,500,"Failed to fetch chirps")
		}
		chirps:=make([]Chirp,len(dbChirps))

		for i,dbChirp:= range dbChirps{
			chirps[i]=Chirp{
				ID: dbChirp.ID,
				CreatedAt: dbChirp.CreatedAt,
				UpdatedAt: dbChirp.UpdatedAt,
				Body: dbChirp.Body,
				UserID: dbChirp.UserID,
			}
		}

		respondWithJSON(w,200,chirps)
	})

	serverErr:=server.ListenAndServe()
	if serverErr!=nil{
		fmt.Printf("Error in starting sever: %v",serverErr)
	}

}