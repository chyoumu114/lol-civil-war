package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/chyoumu114/lol-civil-war/internal/domain/champion"
	"github.com/chyoumu114/lol-civil-war/internal/domain/rank"
	"github.com/chyoumu114/lol-civil-war/internal/service"
	"github.com/chyoumu114/lol-civil-war/internal/store"
)

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.App.Meta())
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	payload := service.MeView{DevLogin: s.Cfg.DevLogin}
	if userID, ok := userIDFrom(r); ok {
		user, err := s.App.St.UserByID(r.Context(), userID)
		if err == nil {
			payload.Authenticated = true
			payload.User = &service.UserView{ID: user.ID, Name: user.Name, Avatar: user.Avatar, IsDev: user.IsDev}
		} else if err != nil && !isStoreNotFound(err) {
			writeError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handlePower(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	position := strings.TrimSpace(query.Get("position"))
	region := strings.TrimSpace(query.Get("region"))
	tier := strings.TrimSpace(query.Get("tier"))
	if position == "" {
		writeValidation(w, "position", "分路不能为空")
		return
	}
	if region == "" {
		writeValidation(w, "region", "大区不能为空")
		return
	}
	if tier == "" {
		writeValidation(w, "tier", "段位不能为空")
		return
	}
	if len([]rune(region)) > 32 {
		writeValidation(w, "region", "大区名称最多 32 个字符")
		return
	}
	if len([]rune(tier)) > 16 {
		writeValidation(w, "tier", "段位名称最多 16 个字符")
		return
	}

	var division *int
	if raw := strings.TrimSpace(query.Get("division")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeValidation(w, "division", "小段必须是整数")
			return
		}
		division = &value
	}
	lp := 0
	if raw := strings.TrimSpace(query.Get("lp")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeValidation(w, "lp", "分数必须是整数")
			return
		}
		lp = value
	}

	view, err := s.App.CalculatePower(service.PowerInput{
		Position: position, Region: region, Tier: tier, Division: division, LP: lp,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleChampionLibrary(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	rawPosition := strings.TrimSpace(r.URL.Query().Get("position"))

	var position *rank.Position
	if rawPosition != "" && !strings.EqualFold(rawPosition, "ALL") {
		parsed, ok := rank.ParsePosition(rawPosition)
		if !ok {
			writeMessage(w, http.StatusUnprocessableEntity, "未知位置："+rawPosition)
			return
		}
		position = &parsed
	}

	result, err := champion.Search(query, position)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleChampionNames(w http.ResponseWriter, r *http.Request) {
	view := s.App.DDragon.Names(r.Context(), r.URL.Query().Get("version"))
	writeJSON(w, http.StatusOK, view)
}

func isStoreNotFound(err error) bool { return errors.Is(err, store.ErrNotFound) }
