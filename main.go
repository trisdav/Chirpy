package main
import (
	"fmt"
	"net/http"
	"sync/atomic"
	"encoding/json"
	"regexp"
	"github.com/joho/godotenv"
	"os"
	"database/sql"
	_ "github.com/lib/pq"
	"tristan-davis.com/internal/database"
	"tristan-davis.com/internal/auth"
	"github.com/google/uuid"
	"time"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries *database.Queries
	BearerToken string
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
	Token    string     `json:"token"`
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r * http.Request) {
			cfg.fileserverHits.Add(1)
			next.ServeHTTP(w,r)
		})
}

func (cfg *apiConfig) metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r * http.Request) {
			w.Header().Set("Content-Type","text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)

			w.Write([]byte(fmt.Sprintf("<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %v times!</p></body></html>\n", cfg.fileserverHits.Load())))
		})
}

func (cfg *apiConfig) reset(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r * http.Request) {
			w.Header().Set("Content-Type","text/plain; charset=utf-8")
			cfg.fileserverHits.Store(0)
			err := cfg.dbQueries.ResetChirps(r.Context())
			if (err != nil ){
				w.WriteHeader(500)
				w.Write([]byte(fmt.Sprintf("Error with reset query: %v",err.Error())))
				return		
			}

			err = cfg.dbQueries.ResetUsers(r.Context())
			if (err != nil ){
				w.WriteHeader(500)
				w.Write([]byte(fmt.Sprintf("Error with reset query: %v",err.Error())))
				return		
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK\n"))
		})
}

func healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type","text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}

func badWordCheck(m string) string {
	re := regexp.MustCompile(`(?i)kerfuffle|sharbert|fornax`)
	result := re.ReplaceAllString(m,"****")
	return result
}

func (cfg *apiConfig) users(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r * http.Request) {
			// DECODE json with the worlds worst json parser
			decoder := json.NewDecoder(r.Body)
			type parameters struct {
				Password string `json:"password"`
				Email string `json:"email"`
			}
			params := parameters{}
			err := decoder.Decode(&params)
			if err != nil {
				w.Header().Set("Content-Type","text/plain")
				w.WriteHeader(500)
				w.Write([]byte("Error decoding params"))
				return
			}
			hashed,hashErr := auth.HashPassword(params.Password)
			fmt.Println(hashed)
			if (hashErr != nil ) {
				w.Header().Set("Content-Type","text/plain")
				w.WriteHeader(500)
				w.Write([]byte("Error hashing password on user creation"))
				return
			}
			type createArg struct {
				Email string
				HashedPassword string
			}
			args := new(database.CreateUserParams)
			args.Email = params.Email
			args.HashedPassword = hashed
			user, err2 := cfg.dbQueries.CreateUser(r.Context(), *args)
			if err2 != nil {
				w.Header().Set("Content-Type","text/plain")
				w.WriteHeader(500)
				w.Write([]byte("Error creating user"))
				return
			}
			userStruct := new(User)
			userStruct.ID = user.ID
			if user.CreatedAt.Valid {
				userStruct.CreatedAt = user.CreatedAt.Time
			}
			if user.UpdatedAt.Valid {
				userStruct.UpdatedAt = user.UpdatedAt.Time
			}
			userStruct.Email = user.Email
			json, _ := json.Marshal(userStruct)
			w.Header().Set("Content-Type","application/json")
			w.WriteHeader(201)
			w.Write(json)
		})
}

func (cfg *apiConfig) chirps(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			bToken, berr := auth.GetBearerToken(r.Header)
			if (berr != nil) {
				w.WriteHeader(401)
				w.Write([]byte(`{"error":"No bearer token"}`))
				return
			}

			id,ierr := auth.ValidateJWT(bToken, cfg.BearerToken)

			if (ierr != nil || id == uuid.Nil ) {
				w.WriteHeader(401)
				w.Write([]byte(`{"error":"Invalid bearer token"}`))
				return
			}

			w.Header().Set("Content-Type","application/json")
			type parameters struct {
				Body string `json:"body"`
			}

			decoder := json.NewDecoder(r.Body)
			params := parameters{}
			err := decoder.Decode(&params)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"Something went wrong"}`))
				return
			}

			maxChirp := 140
			if len(params.Body) > maxChirp {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"Chirp is too long"}`))
				return
			}
			cParams := database.CreateChirpParams{}
			cParams.Body = badWordCheck(params.Body)
			cParams.UserID = id

			chirp, err2 := cfg.dbQueries.CreateChirp(r.Context(), cParams)
			if err2 != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"Failed to create chirp in db"}`))
				return
			}
			chirpStruct := new(Chirp)
			chirpStruct.ID = chirp.ID
			if chirp.CreatedAt.Valid {
				chirpStruct.CreatedAt = chirp.CreatedAt.Time
			}
			if chirp.UpdatedAt.Valid {
				chirpStruct.UpdatedAt = chirp.UpdatedAt.Time
			}
			chirpStruct.Body = chirp.Body
			chirpStruct.UserID = chirp.UserID
			json, _ := json.Marshal(chirpStruct)
			w.WriteHeader(201)
			w.Write(json)
		})
}

func (cfg *apiConfig) login(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r * http.Request) {
			decoder := json.NewDecoder(r.Body)
			type parameters struct {
				Password string `json:"password"`
				Email string `json:"email"`
				ExpiresInSeconds time.Duration `json:"expires_in_seconds"`
			}
			params := parameters{}
			err := decoder.Decode(&params)
			if err != nil {
				fmt.Println("Error decoding params")
			}
			if (params.ExpiresInSeconds == 0) {
				params.ExpiresInSeconds=time.Minute * 60;
			}
			//to do
			user, hErr := cfg.dbQueries.GetUser(r.Context(), params.Email)
			if (hErr != nil) {
				w.WriteHeader(401)
				w.Write([]byte(`{"error":"Invalid credentials (email doesn't exist)"}`))
				return
			}
			checkErr := auth.CheckPasswordHash(user.HashedPassword,params.Password)
			if (checkErr != nil) {
				w.WriteHeader(401)
				w.Write([]byte(`{"error":"Invalid credentials"}`))
				return
			}

			token, terr := auth.MakeJWT(user.ID, cfg.BearerToken, params.ExpiresInSeconds)
			if (terr != nil) {
				w.WriteHeader(401)
				w.Write([]byte(`{"error":"Failed to create token"}`))
				return
			}

			userStruct := new(User)
			userStruct.ID = user.ID
			if user.CreatedAt.Valid {
				userStruct.CreatedAt = user.CreatedAt.Time
			}
			if user.UpdatedAt.Valid {
				userStruct.UpdatedAt = user.UpdatedAt.Time
			}
			userStruct.Email = user.Email
			userStruct.Token = token
			json, _ := json.Marshal(userStruct)
			w.Header().Set("Content-Type","application/json")
			w.WriteHeader(200)
			w.Write(json)
		})
}


func (cfg *apiConfig) getChirps(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type","application/json")

			chirps, err := cfg.dbQueries.SelectChirps(r.Context())
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"Failed to select chirps in db"}`))
				return
			}

			type jsonFormatter struct {
				ID uuid.UUID `json:"id"`
				Body string `json:"body"`
				CreatedAt time.Time `json:"created_at"`
				UpdatedAt time.Time `json:"updated_at"`
				UserID uuid.UUID `json:"user_id"`
			}
			finalList := []jsonFormatter{}

			for _, chirp := range chirps {
				newChirp := new(jsonFormatter)
				newChirp.ID = chirp.ID
				newChirp.Body = chirp.Body
				newChirp.CreatedAt = chirp.CreatedAt.Time
				newChirp.UpdatedAt = chirp.UpdatedAt.Time
				newChirp.UserID = chirp.UserID
				finalList = append(finalList, *newChirp)
			}

			json, err2 := json.Marshal(finalList)
			if (err2 != nil) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"Failed to parse chirps from db"}`))
				return
			}
			w.WriteHeader(200)
			w.Write(json)
		})
}



func (cfg *apiConfig) getChirp(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bToken, berr := auth.GetBearerToken(r.Header)
			if (berr != nil) {
				w.WriteHeader(401)
				w.Write([]byte(`{"error":"No bearer token"}`))
				return
			}

			id,ierr := auth.ValidateJWT(bToken, cfg.BearerToken)

			if (ierr != nil || id == uuid.Nil ) {
				w.WriteHeader(401)
				w.Write([]byte(`{"error":"Invalid bearer token"}`))
				return
			}

			w.Header().Set("Content-Type","application/json")

			chirpUuid,uerr := uuid.Parse(r.PathValue("chirp_id"))
			if (uerr != nil ) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"Failed to select chirp in db, invalid id"}`))
				return				
			}

			chirp, err := cfg.dbQueries.GetChirp(r.Context(),chirpUuid)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"Failed to select chirp in db"}`))
				return
			}

			type jsonFormatter struct {
				ID uuid.UUID `json:"id"`
				Body string `json:"body"`
				CreatedAt time.Time `json:"created_at"`
				UpdatedAt time.Time `json:"updated_at"`
				UserID uuid.UUID `json:"user_id"`
			}

			newChirp := new(jsonFormatter)
			newChirp.ID = chirp.ID
			newChirp.Body = chirp.Body
			newChirp.CreatedAt = chirp.CreatedAt.Time
			newChirp.UpdatedAt = chirp.UpdatedAt.Time
			newChirp.UserID = chirp.UserID

			json, err2 := json.Marshal(newChirp)
			if (err2 != nil) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"Failed to parse chirp from db"}`))
				return
			}
			w.WriteHeader(200)
			w.Write(json)
		})
}


func main() {
	// db url
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	bearerToken := os.Getenv("BEARER_TOKEN")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Println("Failed to connect to database")
		return
	}
	dbQueries := database.New(db)
	
	// basic config
	fs := http.FileServer(http.Dir(".")) 
	apiCfg:=new(apiConfig)
	apiCfg.dbQueries = dbQueries
	apiCfg.BearerToken = bearerToken

	
	mux:=http.NewServeMux()
	
	// db pages
	mux.Handle("POST /api/users",apiCfg.users(http.StripPrefix("/api/users/",fs)))
	mux.Handle("POST /api/chirps",apiCfg.chirps(http.StripPrefix("/api/chirps/",fs)))
	mux.Handle("GET /api/chirps",apiCfg.getChirps(http.StripPrefix("/api/chirps/",fs)))
	mux.Handle("GET /api/chirps/{chirp_id}",apiCfg.getChirp(http.StripPrefix("/api/chirps/{chirp_id}",fs)))

	// Auth pages
	mux.Handle("POST /api/login",apiCfg.login(http.StripPrefix("/api/login",fs)))

	// initial pages
	mux.HandleFunc("GET /api/healthz",healthz)
	mux.Handle("/app/",apiCfg.middlewareMetricsInc(http.StripPrefix("/app/",fs)))
	mux.Handle("GET /admin/metrics",apiCfg.metrics(fs))
	//mux.Handle("/reset/",apiCfg.reset(fs))
	mux.Handle("POST /admin/reset",apiCfg.reset(http.StripPrefix("/admin/reset",fs)))
	server:=&http.Server {
		Addr:":8888",
		Handler: mux,
	}
	server.ListenAndServe()
}
