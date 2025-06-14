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
	"github.com/google/uuid"
	"time"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries *database.Queries
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
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
	fmt.Println(m)
	result := re.ReplaceAllString(m,"****")
	fmt.Println(result)
	return result
}

func (cfg *apiConfig) users(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r * http.Request) {
			// DECODE json with the worlds worst json parser
			decoder := json.NewDecoder(r.Body)
			type parameters struct {
				Email string `json:"email"`
			}
			params := parameters{}
			err := decoder.Decode(&params)
			if err != nil {
				fmt.Println("Error decoding params")
			}
			user, err2 := cfg.dbQueries.CreateUser(r.Context(), params.Email)
			if err2 != nil {
				fmt.Println("Error in users db query")
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
			w.Header().Set("Content-Type","application/json")
			type parameters struct {
				Body string `json:"body"`
				UserID uuid.UUID `json:"user_id"`
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
			cParams.UserID = params.UserID

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


func main() {
	// db url
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
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

	
	mux:=http.NewServeMux()
	
	// db pages
	mux.Handle("POST /api/users",apiCfg.users(http.StripPrefix("/api/users/",fs)))
	mux.Handle("POST /api/chirps",apiCfg.chirps(http.StripPrefix("/api/chirps/",fs)))

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
