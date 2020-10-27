package websocket

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	ws "github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
	"time"
)

type Config struct {
	Port int
}

var upgrader = ws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Logger interface {
	Info(...interface{})
	Error(...interface{})
	Infof(string, ...interface{})
	Debugf(string, ...interface{})
}

func New(cfg Config, logger Logger) *Server {
	srv := &Server{
		log: logger,
		http: &http.Server{
			Addr: fmt.Sprintf(":%d", cfg.Port),
		},
		hub: newHub(),
	}
	srv.http.Handler = srv.buildHandler()

	return srv
}

type Server struct {
	http *http.Server
	hub  *Hub
	log  Logger
}

func (s *Server) buildHandler() http.Handler {
	router := mux.NewRouter()

	router.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}
		startClient(s.hub, conn, s.log, 256)
	})

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

	go s.hub.run()

	go func() {
		<-ctx.Done()
		sdCtx, _ := context.WithTimeout(context.Background(), 5*time.Second)
		err := s.http.Shutdown(sdCtx)
		if err != nil {
			s.log.Infof("http server shutdown (", err.Error(), ")")
		}
	}()
}
