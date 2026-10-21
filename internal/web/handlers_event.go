package web

import (
	"net/http"
	"strconv"

	"github.com/chyoumu114/lol-civil-war/internal/service"
	"github.com/chyoumu114/lol-civil-war/internal/store"
)

func (s *Server) handleEventList(w http.ResponseWriter, r *http.Request, group store.Group, member store.Member) {
	query := r.URL.Query()
	page := 1
	if raw := query.Get("page"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			page = value
		}
	}
	if page < 1 {
		page = 1
	}
	pageSize := 20
	if raw := query.Get("page_size"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			pageSize = value
		} else {
			pageSize = 1
		}
	}
	if pageSize < 1 {
		pageSize = 1
	}
	if pageSize > 50 {
		pageSize = 50
	}

	result, err := s.App.ListEvents(r.Context(), group, query.Get("status"), page, pageSize)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleEventCreate(w http.ResponseWriter, r *http.Request, group store.Group, member store.Member) {
	var payload service.EventPayload
	if err := decodeJSON(w, r, &payload, 1<<20); err != nil {
		writeMessage(w, http.StatusUnprocessableEntity, "请求格式不正确")
		return
	}
	view, err := s.App.CreateEvent(r.Context(), group, member, payload)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"event": view})
}

func (s *Server) handleEventShow(w http.ResponseWriter, r *http.Request, group store.Group, event store.Event, member store.Member) {
	detail, err := s.App.EventDetail(r.Context(), event, member)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleEventUpdate(w http.ResponseWriter, r *http.Request, group store.Group, event store.Event, member store.Member) {
	var payload service.EventPayload
	if err := decodeJSON(w, r, &payload, 1<<20); err != nil {
		writeMessage(w, http.StatusUnprocessableEntity, "请求格式不正确")
		return
	}
	view, err := s.App.UpdateEvent(r.Context(), event, member, payload)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"event": view})
}

func (s *Server) handleEventEnd(w http.ResponseWriter, r *http.Request, group store.Group, event store.Event, member store.Member) {
	detail, err := s.App.EndEvent(r.Context(), event, member)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleEventSpectator(w http.ResponseWriter, r *http.Request, group store.Group, event store.Event, member store.Member) {
	var payload struct {
		Watching bool `json:"watching"`
	}
	if err := decodeJSON(w, r, &payload, 1<<20); err != nil {
		writeMessage(w, http.StatusUnprocessableEntity, "请求格式不正确")
		return
	}
	if err := s.App.SetSpectator(r.Context(), event, member, payload.Watching); err != nil {
		writeError(w, err)
		return
	}
	detail, err := s.App.EventDetail(r.Context(), event, member)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}
