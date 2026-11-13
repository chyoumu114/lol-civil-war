package web

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDecodeJSONRejectsTrailingData 验证 V-16：同一请求体不能夹带多个 JSON 对象。
func TestDecodeJSONRejectsTrailingData(t *testing.T) {
	cases := []struct {
		name string
		body string
		ok   bool
	}{
		{name: "单对象", body: `{"a":1}`, ok: true},
		{name: "尾随对象", body: `{"a":1}{"b":2}`, ok: false},
		{name: "尾随垃圾", body: `{"a":1}garbage`, ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			var dst map[string]any
			err := decodeJSON(rec, req, &dst, 1024)
			if tc.ok && err != nil {
				t.Fatalf("合法请求被拒绝: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("尾随数据应被拒绝")
			}
		})
	}
}
