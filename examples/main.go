package main

import (
	"log/slog"
	"net/http"

	"github.com/matt-hogan/mux"
	"github.com/matt-hogan/mux/middleware"
)

func main() {
	router := mux.NewRouter(
		mux.WithLogLevel(slog.LevelDebug),
	)
	router.Use(
		middleware.Logging,
		middleware.Recovery,
	)

	router.Get("/", func(c *mux.Context) error {
		return c.HTML(http.StatusOK, "coming soon")
	})

	test := router.Group("/test")
	test.Get("", func(c *mux.Context) error {
		return c.HTML(http.StatusOK, "test")
	})
	test.Get("/health", func(c *mux.Context) error {
		return c.HTML(http.StatusOK, "health")
	})
	test.Get("/{id}", func(c *mux.Context) error {
		return c.HTML(http.StatusOK, c.Request.PathValue("id"))
	})

	router.Start()
}
