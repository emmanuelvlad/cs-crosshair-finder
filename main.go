package main

import (
	"net/http"
	"os"

	"aimfrag/xhair/handlers"

	muxHandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	cors := muxHandlers.CORS(
		muxHandlers.AllowedHeaders([]string{"content-type"}),
		muxHandlers.AllowedOrigins([]string{"*"}),
		muxHandlers.AllowCredentials(),
	)
	router := mux.NewRouter()
	router.HandleFunc("/{playerID}", handlers.CrosshairHandler)

	router.Use(cors)

	http.Handle("/", router)

	port, exists := os.LookupEnv("PORT")
	if !exists {
		port = "3500"
	}

	http.ListenAndServe(":"+port, nil)
}
