package httpserver

import "net/http"

func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /api/state", s.handleState)

	mux.Handle("GET /ws", s.wsHandler)
	mux.Handle("GET /video/stream", s.cameraHandler)

	fileServer := http.FileServer(http.Dir(s.cfg.Web.StaticDir))
	mux.Handle("GET /", fileServer)
}