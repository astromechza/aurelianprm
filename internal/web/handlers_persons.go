package web

import "net/http"

func (s *Server) handlePersonsList(w http.ResponseWriter, r *http.Request)     {}
func (s *Server) handlePersonsNew(w http.ResponseWriter, r *http.Request)      {}
func (s *Server) handlePersonsCreate(w http.ResponseWriter, r *http.Request)   {}
func (s *Server) handlePersonsDetail(w http.ResponseWriter, r *http.Request)   {}
func (s *Server) handlePersonsUpdate(w http.ResponseWriter, r *http.Request)   {}
func (s *Server) handlePersonsDelete(w http.ResponseWriter, r *http.Request)   {}
func (s *Server) handlePersonsEditForm(w http.ResponseWriter, r *http.Request) {}
