package web

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chyoumu114/lol-civil-war/internal/service"
	"github.com/chyoumu114/lol-civil-war/internal/store"
)

func (s *Server) handleMatchCreate(w http.ResponseWriter, r *http.Request, group store.Group, event store.Event, member store.Member) {
	created, err := s.App.CreateMatch(r.Context(), event, member)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"match": created})
}

func (s *Server) handleMatchShow(w http.ResponseWriter, r *http.Request, group store.Group, event store.Event, member store.Member) {
	detail, err := s.App.ShowMatch(r.Context(), event, s.gameNoFromPath(r), member)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleMatchSave(w http.ResponseWriter, r *http.Request, group store.Group, event store.Event, member store.Member) {
	var payload service.MatchSavePayload
	if err := decodeJSON(w, r, &payload, 8<<20); err != nil {
		writeMessage(w, http.StatusUnprocessableEntity, "请求格式不正确")
		return
	}
	detail, err := s.App.SaveMatch(r.Context(), event, s.gameNoFromPath(r), member, payload)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleMatchDelete(w http.ResponseWriter, r *http.Request, group store.Group, event store.Event, member store.Member) {
	if err := s.App.DeleteMatch(r.Context(), event, s.gameNoFromPath(r), member); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMatchRate(w http.ResponseWriter, r *http.Request, group store.Group, event store.Event, member store.Member) {
	var payload struct {
		TargetMemberID int64 `json:"targetMemberId"`
		Score          int   `json:"score"`
	}
	if err := decodeJSON(w, r, &payload, 1<<20); err != nil {
		writeMessage(w, http.StatusUnprocessableEntity, "请求格式不正确")
		return
	}
	summary, err := s.App.Rate(r.Context(), event, s.gameNoFromPath(r), member, payload.TargetMemberID, payload.Score)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// handleRoflUpload 接收 .rofl，做分层限流（用户/全局并发）后交给 service 解析（M-07）。
func (s *Server) handleRoflUpload(w http.ResponseWriter, r *http.Request, group store.Group, event store.Event, member store.Member) {
	// 上传是重操作：单独限速，避免账号级 120/min 额度被大文件占满。
	if allowed, retry := s.limiter.allow("rofl:"+itoa64(member.UserID), 5, time.Minute); !allowed {
		w.Header().Set("Retry-After", itoa(retry))
		auditLog("rate_limited", "scope", "rofl", "userNo", itoa64(member.UserID), "path", r.URL.Path)
		writeMessage(w, http.StatusTooManyRequests, "上传过于频繁，请稍后重试")
		return
	}
	// 全局并发配额：满载时快速失败，避免磁盘与内存同时被多个大文件放大。
	select {
	case s.uploadSem <- struct{}{}:
		defer func() { <-s.uploadSem }()
	default:
		w.Header().Set("Retry-After", "10")
		writeMessage(w, http.StatusTooManyRequests, "当前上传任务较多，请稍后重试")
		return
	}

	// 文件本体 + 少量 multipart 头部开销。
	maxBytes := s.Cfg.RoflMaxKB*1024 + (1 << 20)
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeMessage(w, http.StatusUnprocessableEntity, "录像文件过大，最大支持 "+itoa64(s.Cfg.RoflMaxKB)+" KB")
			return
		}
		writeMessage(w, http.StatusUnprocessableEntity, "请选择要上传的 .rofl 文件")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeMessage(w, http.StatusUnprocessableEntity, "请选择要上传的 .rofl 文件")
		return
	}
	defer file.Close()
	// 只取文件名，杜绝客户端传入路径片段。
	fileName := filepath.Base(strings.TrimSpace(header.Filename))
	if fileName == "" || fileName == "." || fileName == "/" {
		writeMessage(w, http.StatusUnprocessableEntity, "请选择要上传的 .rofl 文件")
		return
	}

	tmpDir := filepath.Join(s.Cfg.DataDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o750); err != nil {
		writeError(w, err)
		return
	}
	tmp, err := os.CreateTemp(tmpDir, "rofl-*.rofl")
	if err != nil {
		writeError(w, err)
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	size, copyErr := io.Copy(tmp, file)
	closeErr := tmp.Close()
	if copyErr != nil || closeErr != nil {
		writeMessage(w, http.StatusUnprocessableEntity, "无法读取上传的录像文件")
		return
	}

	result, err := s.App.ImportRofl(r.Context(), event, s.gameNoFromPath(r), member, service.UploadedFile{
		Path: tmpPath,
		Name: fileName,
		Size: size,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
