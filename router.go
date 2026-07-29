package mux

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"
)

type HandlerFunc func(*Context) error

type Middleware func(HandlerFunc) HandlerFunc

type router struct {
	*http.ServeMux
	middleware []Middleware
	logger     *slog.Logger
	prefix     string
	server     *http.Server
}

type routerOpt func(*router)

func OnPort(port string) routerOpt {
	return func(r *router) {
		r.server.Addr = ":" + port
	}
}

func WithLogger(logger *slog.Logger) routerOpt {
	return func(r *router) {
		r.logger = logger
	}
}

func WithLogLevel(level slog.Level) routerOpt {
	return func(r *router) {
		r.logger = routerLogger(level)
	}
}

func WithServer(server *http.Server) routerOpt {
	return func(r *router) {
		r.server = server
	}
}

func WithReadTimeout(timeout time.Duration) routerOpt {
	return func(r *router) {
		r.server.ReadTimeout = timeout
	}
}

func WithWriteTimeout(timeout time.Duration) routerOpt {
	return func(r *router) {
		r.server.WriteTimeout = timeout
	}
}

// NewRouter create a new router instance with the config loaded from
// environment variables.
func NewRouter(opts ...routerOpt) *router {
	r := &router{
		ServeMux: http.NewServeMux(),
		server: &http.Server{
			Addr:           ":8080",
			MaxHeaderBytes: 1 << 20,
			ReadTimeout:    5 * time.Second,
			WriteTimeout:   10 * time.Second,
		},
		logger: routerLogger(slog.LevelInfo),
	}
	for _, opt := range opts {
		opt(r)
	}
	r.server.Handler = r
	return r
}

func routerLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

// Start runs the HTTP server and gracefully shuts down on termination signals.
func (r *router) Start() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		r.logger.Info("starting server", slog.String("addr", r.server.Addr))
		if err := r.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			r.logger.Error("server error", slog.String("error", err.Error()))
			return
		}
		r.logger.Info("stopped serving new connections")
	}()

	<-stop
	r.logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := r.server.Shutdown(ctx); err != nil {
		r.logger.Error("server shutdown error", slog.String("error", err.Error()))
	} else {
		r.logger.Info("server gracefully shutdown")
	}
}

// Group creates a new router group with prefix and optional middleware.
func (r *router) Group(group string, middleware ...Middleware) *router {
	groupRouter := *r
	groupRouter.prefix = strings.TrimRight(r.prefix, "/") + "/" + strings.Trim(group, "/")
	groupRouter.middleware = slices.Concat(r.middleware, middleware)
	return &groupRouter
}

// Use adds middleware to the chain.
func (r *router) Use(middleware ...Middleware) {
	r.middleware = append(r.middleware, middleware...)
}

// applyMiddleware returns a new handler wrapped with middleware functions.
// Middleware are applied so they are ran in the same order they are set.
func (r *router) applyMiddleware(handlerFunc HandlerFunc, middleware ...Middleware) HandlerFunc {
	for i := len(r.middleware) - 1; i >= 0; i-- {
		handlerFunc = r.middleware[i](handlerFunc)
	}
	for _, m := range middleware {
		handlerFunc = m(handlerFunc)
	}
	return func(c *Context) error {
		return handlerFunc(c)
	}
}

// Request registers a new route for an optional method and path with optional middleware.
func (r *router) Request(path string, handlerFunc HandlerFunc, middleware ...Middleware) {
	handlerFunc = r.applyMiddleware(handlerFunc, middleware...)

	method, route, ok := strings.Cut(path, " ")
	if !ok {
		route, method = method, ""
	}

	if r.prefix != "" {
		prefix := strings.TrimRight(r.prefix, "/")
		if route == "" {
			// Empty group route maps to the prefix itself; "/" stays reserved for directory routes.
			route = prefix
		} else {
			route = prefix + "/" + strings.TrimLeft(route, "/")
		}
	} else if route == "/" {
		// ServeMux "/" is a subtree match. Use exact-root form so unmatched paths return 404.
		route = "/{$}"
	}

	pattern := route
	if method != "" {
		pattern = method + " " + route
	}

	r.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
		c := &Context{
			Request: req,
			Response: Response{
				Writer:     w,
				StatusCode: http.StatusOK,
			},
			logger: r.logger.With(
				slog.String("method", req.Method),
				slog.String("path", req.URL.Path),
			),
		}

		if err := handlerFunc(c); err != nil {
			c.Logger().Error("failed handling request", slog.String("error", err.Error()))
			if !c.Response.Written {
				c.Error(http.StatusInternalServerError)
			}
		}
	})

}

// Get registers a new get route for a path with optional middleware.
func (r *router) Get(path string, handlerFunc HandlerFunc, middleware ...Middleware) {
	r.Request(http.MethodGet+" "+path, handlerFunc, middleware...)
}

// Post registers a new post route for a path with optional middleware.
func (r *router) Post(path string, handlerFunc HandlerFunc, middleware ...Middleware) {
	r.Request(http.MethodPost+" "+path, handlerFunc, middleware...)
}

// Put registers a new put route for a path with optional middleware.
func (r *router) Put(path string, handlerFunc HandlerFunc, middleware ...Middleware) {
	r.Request(http.MethodPut+" "+path, handlerFunc, middleware...)
}

// Patch registers a new patch route for a path with optional middleware.
func (r *router) Patch(path string, handlerFunc HandlerFunc, middleware ...Middleware) {
	r.Request(http.MethodPatch+" "+path, handlerFunc, middleware...)
}

// Delete registers a new delete route for a path with optional middleware.
func (r *router) Delete(path string, handlerFunc HandlerFunc, middleware ...Middleware) {
	r.Request(http.MethodDelete+" "+path, handlerFunc, middleware...)
}
