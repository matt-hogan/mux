package mux

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

type ContextKey string

type Context struct {
	Request  *http.Request
	Response Response
	logger   *slog.Logger
}

type Response struct {
	Writer     http.ResponseWriter
	StatusCode int
	Written    bool
}

// Cookie retrieves a cookie from the request by name.
func (c *Context) Cookie(name string) (*http.Cookie, error) {
	return c.Request.Cookie(name)
}

// Error responds to a request with the provided error status.
func (c *Context) Error(status int) {
	if c.Response.Written {
		c.Logger().Error("failed to write error response; response already written",
			slog.Int("status", status),
			slog.Int("written_status", c.Response.StatusCode),
		)
		return
	}
	c.Response.StatusCode = status
	c.Response.Written = true
	http.Error(c.Response.Writer, http.StatusText(status), status)
}

// Get retrieves a value from the context.
func (c *Context) Get(key ContextKey) any {
	return c.Request.Context().Value(key)
}

// HTML sends an HTML response with a status.
func (c *Context) HTML(status int, body string) error {
	c.WithContentType(MIMETextHTML)
	c.WithStatus(status)

	_, err := io.WriteString(c.Response.Writer, body)
	return err
}

// JSON sends a JSON response with a status.
func (c *Context) JSON(status int, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	c.WithContentType(MIMEApplicationJSON)
	c.WithStatus(status)

	_, err = c.Response.Writer.Write(data)
	return err
}

// Logger returns the logger for the context.
func (c *Context) Logger() *slog.Logger {
	if c.logger == nil {
		return slog.Default()
	}
	return c.logger
}

// Redirect redirects to a url with a status.
func (c *Context) Redirect(status int, path string) {
	c.Response.StatusCode = status
	c.Response.Written = true
	http.Redirect(c.Response.Writer, c.Request, path, status)
}

// Set saves a value to the context.
func (c *Context) Set(key ContextKey, value any) {
	ctx := context.WithValue(c.Request.Context(), key, value)
	c.Request = c.Request.WithContext(ctx)
}

// SetCookie sets a cookie on the response.
func (c *Context) SetCookie(cookie *http.Cookie) {
	http.SetCookie(c.Response.Writer, cookie)
}

// Stream sends a streaming response with a status and content type.
func (c *Context) Stream(status int, contentType string, body io.Reader) error {
	c.WithContentType(contentType)
	c.WithStatus(status)

	_, err := io.Copy(c.Response.Writer, body)
	return err
}

// WithStatus sets the http response header with the status code.
func (c *Context) WithStatus(statusCode int) {
	if c.Response.Written {
		return
	}
	c.Response.StatusCode = statusCode
	c.Response.Written = true
	c.Response.Writer.WriteHeader(statusCode)
}

// WithContentType sets the http response header with the content type.
func (c *Context) WithContentType(contentType string) {
	c.Response.Writer.Header().Set("Content-Type", contentType)
}
