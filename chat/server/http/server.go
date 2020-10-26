package http

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
	"sync"
	"time"
)

const staticDir = "./server/http/static/"

type Config struct {
	Port int
}

type Logger interface {
	Info(...interface{})
	Infof(string, ...interface{})
	Debugf(string, ...interface{})
}

func New(cfg Config, logger Logger) *Server {
	srv := &Server{
		log: logger,
		http: &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.Port),
			Handler: buildHandler(),
		},
	}
	return srv
}

type Server struct {
	http *http.Server
	log  Logger
}

func buildHandler() http.Handler {
	router := mux.NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, staticDir+"main.html")
	})

	router.PathPrefix("/static/").
		Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir)))).
		Methods(http.MethodGet)

	return router
}

func (s *Server) Run(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	s.log.Info("http server: begin run")

	go func() {
		defer wg.Done()
		s.log.Debugf("http server: addr=", s.http.Addr)
		err := s.http.ListenAndServe()
		s.log.Infof("http server: end run > ", err)
	}()

	go func() {
		<-ctx.Done()
		sdCtx, _ := context.WithTimeout(context.Background(), 5*time.Second) // nolint
		err := s.http.Shutdown(sdCtx)
		if err != nil {
			s.log.Infof("http server shutdown (", err.Error(), ")")
		}
	}()
}
