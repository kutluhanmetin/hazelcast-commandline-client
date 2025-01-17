package errors

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/gohxs/readline"
	"github.com/hazelcast/hazelcast-go-client/hzerrors"
)

func IsUserCancelled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, ErrUserCancelled) || errors.Is(err, readline.ErrInterrupt)
}

func IsTimeout(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, hzerrors.ErrTimeout)
}

func MakeString(err error) string {
	if IsTimeout(err) {
		return "TIMEOUT"
	}
	var httpErr HTTPError
	var errStr string
	if errors.As(err, &httpErr) {
		errStr = makeErrorStringFromHTTPResponse(httpErr.Text())
	} else {
		errStr = err.Error()
	}
	// convert the first character of the error string to upper case
	if len(errStr) > 0 {
		r := []rune(errStr)
		if r[0] >= 'a' && r[0] <= 'z' {
			r[0] -= 'a' - 'A'
		}
		errStr = string(r)
	}
	return "ERROR " + errStr
}

func makeErrorStringFromHTTPResponse(text string) string {
	m := map[string]any{}
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		return text
	}
	if v, ok := m["errorCode"]; ok {
		if v == "ClusterTokenNotFound" {
			return "Discovery token is not valid for this cluster"
		}
	}
	if v, ok := m["message"]; ok {
		if vs, ok := v.(string); ok {
			return vs
		}
	}
	return text
}
