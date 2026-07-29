package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/matt-hogan/mux"
)

// Logging is a middleware to log HTTP requests.
func Logging(next mux.HandlerFunc) mux.HandlerFunc {
	return mux.HandlerFunc(func(c *mux.Context) error {
		start := time.Now().UTC()
		err := next(c)
		if err != nil && !c.Response.Written {
			c.Error(http.StatusInternalServerError)
		}
		c.Logger().Info("http request handled",
			slog.String("user_agent", c.Request.UserAgent()),
			slog.String("client_ip", c.Request.RemoteAddr), // TODO: get real ip
			slog.Group("response",
				slog.Int("status", c.Response.StatusCode),
				slog.String("duration", time.Since(start).String()),
			),
		)
		return err
	})
}

// Recovery is a middleware to recover from panics and respond with a 500 error
func Recovery(next mux.HandlerFunc) mux.HandlerFunc {
	return mux.HandlerFunc(func(c *mux.Context) (err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				c.Logger().Error("recovered from panic during request",
					"error", recovered,
					"trace", string(debug.Stack()),
				)
				c.Error(http.StatusInternalServerError)
				err = nil
			}
		}()
		return next(c)
	})
}
