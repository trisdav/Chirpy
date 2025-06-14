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
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK\n"))
		})
}

func healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type","text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}



func validateChrip(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type","text/plain; charset=utf-8")
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

	cleanString := badWordCheck(params.Body)
	// Go's json library is so jank, why would I use it?
	responseString:=fmt.Sprintf("{\"cleaned_body\":\"%v\"}", cleanString)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(responseString))
}

func badWordCheck(m string) string {
	re := regexp.MustCompile(`(?i)kerfuffle|sharbert|fornax`)
	fmt.Println(m)
	result := re.ReplaceAllString(m,"****")
	fmt.Println(result)
	return result
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
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

	// initial pages
	mux.HandleFunc("GET /api/healthz",healthz)
	mux.HandleFunc("POST /api/validate_chirp",validateChrip)
	mux.Handle("/app/",apiCfg.middlewareMetricsInc(http.StripPrefix("/app/",fs)))
	mux.Handle("GET /admin/metrics",apiCfg.metrics(fs))
	//mux.Handle("/reset/",apiCfg.reset(fs))
	mux.HandleFunc("POST /admin/reset",func(w http.ResponseWriter, r * http.Request) {
			w.Header().Set("Content-Type","text/plain; charset=utf-8")
			apiCfg.fileserverHits.Store(0)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK\n"))
		})
	server:=&http.Server {
		Addr:":8888",
		Handler: mux,
	}
	server.ListenAndServe()
}
