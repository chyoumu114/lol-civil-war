package web

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/chyoumu114/lol-civil-war/internal/service"
	"github.com/chyoumu114/lol-civil-war/internal/store"
)

var groupNumberPattern = regexp.MustCompile(`^\d{5,12}$`)

func (s *Server) handleGroupResolve(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		GroupNumber string `json:"groupNumber"`
	}
	if err := decodeJSON(w, r, &payload, 1<<20); err != nil {
		writeValidation(w, "groupNumber", "请提供 QQ 群号")
		return
	}
	number := strings.TrimSpace(payload.GroupNumber)
	if number == "" {
		writeValidation(w, "groupNumber", "请提供 QQ 群号")
		return
	}
	if !groupNumberPattern.MatchString(number) {
		writeValidation(w, "groupNumber", "群号必须是 5-12 位数字")
		return
	}

	group, err := s.App.ResolveGroupByNumber(r.Context(), number)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"key":  group.PublicKey,
		"name": group.Name,
	})
}

func (s *Server) handleGroupShow(w http.ResponseWriter, r *http.Request, group store.Group, member store.Member) {
	writeJSON(w, http.StatusOK, map[string]any{
		"group": map[string]any{
			"id":   group.ID,
			"key":  group.PublicKey,
			"name": group.Name,
		},
		"member":          s.App.MemberView(member),
		"profileComplete": isProfileCompleteMember(member),
		"meta":            s.App.Meta(),
	})
}

func (s *Server) handleProfileUpdate(w http.ResponseWriter, r *http.Request, group store.Group, member store.Member) {
	var payload service.ProfilePayload
	if err := decodeJSON(w, r, &payload, 1<<20); err != nil {
		writeValidation(w, "nickname", "请求格式不正确")
		return
	}
	if strings.TrimSpace(payload.Nickname) == "" {
		writeValidation(w, "nickname", "昵称不能为空")
		return
	}
	updated, view, err := s.App.UpdateProfile(r.Context(), member, payload)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"member":          view,
		"profileComplete": isProfileCompleteMember(updated),
	})
}

func (s *Server) handleRanking(w http.ResponseWriter, r *http.Request, group store.Group, member store.Member) {
	view, err := s.App.Ranking(r.Context(), group, strings.TrimSpace(r.URL.Query().Get("metric")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handlePlayerAnalysis(w http.ResponseWriter, r *http.Request, group store.Group, member store.Member) {
	memberID, _ := parseID(r.PathValue("member"))
	view, err := s.App.PlayerAnalysis(r.Context(), group, memberID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func isProfileCompleteMember(member store.Member) bool {
	return member.Nickname != "" && member.QQNumber != nil && *member.QQNumber != "" &&
		member.RankTier != nil && *member.RankTier != "" && len(member.Positions) > 0
}

func parseID(raw string) (int64, error) {
	return strconv.ParseInt(raw, 10, 64)
}
