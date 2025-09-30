package client

import (
	"fmt"
	"strings"

	lighterapi "github.com/defi-maker/golighter/api"
)

func resultCodeError(status int, body []byte, rc *lighterapi.ResultCode) error {
	if rc != nil {
		if msg := strings.TrimSpace(deref(rc.Message)); msg != "" {
			return fmt.Errorf("lighter api: code=%d status=%d message=%s", rc.Code, status, msg)
		}
		return fmt.Errorf("lighter api: code=%d status=%d", rc.Code, status)
	}
	return statusError(status, body)
}

func statusError(status int, body []byte) error {
	if len(body) == 0 {
		return fmt.Errorf("lighter api: status=%d", status)
	}
	snippet := strings.TrimSpace(string(body))
	if len(snippet) > 256 {
		snippet = snippet[:256]
	}
	return fmt.Errorf("lighter api: status=%d body=%s", status, snippet)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
