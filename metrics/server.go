package metrics

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
)

type Server struct {
	addr string
}

func NewMetricsServer(addr string) *Server {
	return &Server{
		addr: addr,
	}
}

func (s *Server) Start() error {
	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Starting metrics server on %s", s.addr)
	return http.ListenAndServe(s.addr, nil)
}
