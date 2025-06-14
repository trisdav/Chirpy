package main
import (
	"fmt"
	"net/http"
	"sync/atomic"
	"encoding/json"
	"regexp"
)

type apiConfig struct {
	fileserverHits atomic.Int32
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

func main() {
	mux:=http.NewServeMux()
	apiCfg :=new(apiConfig)
	mux.HandleFunc("GET /api/healthz",healthz)
	mux.HandleFunc("POST /api/validate_chirp",validateChrip)
	fs := http.FileServer(http.Dir(".")) 
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
