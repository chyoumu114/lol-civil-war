package web

import (
	"net/http"

	"github.com/chyoumu114/lol-civil-war/internal/service"
	"github.com/chyoumu114/lol-civil-war/internal/store"
)

func (s *Server) handleRegistrationSave(w http.ResponseWriter, r *http.Request, group store.Group, event store.Event, member store.Member) {
	var payload service.RegistrationPayload
	if err := decodeJSON(w, r, &payload, 1<<20); err != nil {
		writeMessage(w, http.StatusUnprocessableEntity, "请求格式不正确")
		return
	}
	isOrganizer := event.CreatorMemberID == member.ID
	if _, err := s.App.SaveRegistration(r.Context(), event, member, isOrganizer, payload); err != nil {
		writeError(w, err)
		return
	}
	s.writeEventDetail(w, r, event, member)
}

func (s *Server) handleRegistrationUpdate(w http.ResponseWriter, r *http.Request, group store.Group, event store.Event, member store.Member) {
	target, err := s.memberFromPath(r, group)
	if err != nil {
		writeError(w, err)
		return
	}
	isOrganizer := event.CreatorMemberID == member.ID
	if !isOrganizer && target.ID != member.ID {
		writeError(w, service.ErrForbidden("只能修改自己的报名信息"))
		return
	}

	var payload service.RegistrationPayload
	if err := decodeJSON(w, r, &payload, 1<<20); err != nil {
		writeMessage(w, http.StatusUnprocessableEntity, "请求格式不正确")
		return
	}
	if _, err := s.App.SaveRegistration(r.Context(), event, target, isOrganizer, payload); err != nil {
		writeError(w, err)
		return
	}
	s.writeEventDetail(w, r, event, member)
}

func (s *Server) handleRegistrationDelete(w http.ResponseWriter, r *http.Request, group store.Group, event store.Event, member store.Member) {
	target, err := s.memberFromPath(r, group)
	if err != nil {
		writeError(w, err)
		return
	}
	isOrganizer := event.CreatorMemberID == member.ID
	if !isOrganizer && target.ID != member.ID {
		writeError(w, service.ErrForbidden("只能取消自己的报名"))
		return
	}
	if err := s.App.DeleteRegistration(r.Context(), event, member, target, isOrganizer); err != nil {
		writeError(w, err)
		return
	}
	s.writeEventDetail(w, r, event, member)
}

func (s *Server) handleRegistrationSpectator(w http.ResponseWriter, r *http.Request, group store.Group, event store.Event, member store.Member) {
	target, err := s.memberFromPath(r, group)
	if err != nil {
		writeError(w, err)
		return
	}
	var payload struct {
		Spectator bool `json:"spectator"`
	}
	if err := decodeJSON(w, r, &payload, 1<<20); err != nil {
		writeMessage(w, http.StatusUnprocessableEntity, "请求格式不正确")
		return
	}
	if err := s.App.ToggleSpectator(r.Context(), event, member, target, payload.Spectator); err != nil {
		writeError(w, err)
		return
	}
	s.writeEventDetail(w, r, event, member)
}

// writeEventDetail 返回组织详情（报名相关接口统一返回它）。
func (s *Server) writeEventDetail(w http.ResponseWriter, r *http.Request, event store.Event, member store.Member) {
	refreshed, err := s.App.St.EventByGroupAndID(r.Context(), event.GroupID, event.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	detail, err := s.App.EventDetail(r.Context(), refreshed, member)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAutoMatch(w http.ResponseWriter, r *http.Request, group store.Group, event store.Event, member store.Member) {
	result, err := s.App.AutoMatch(r.Context(), event, member)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleRosterApply(w http.ResponseWriter, r *http.Request, group store.Group, event store.Event, member store.Member) {
	var payload service.RosterApplyPayload
	if err := decodeJSON(w, r, &payload, 4<<20); err != nil {
		writeMessage(w, http.StatusUnprocessableEntity, "请求格式不正确")
		return
	}
	roster, err := s.App.ApplyRoster(r.Context(), event, member, payload)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"roster": roster})
}
