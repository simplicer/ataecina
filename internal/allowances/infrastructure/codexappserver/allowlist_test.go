package codexappserver

import (
	"bytes"
	"errors"
	"testing"
)

type buffer struct{ bytes.Buffer }

func (*buffer) Close() error { return nil }

func TestOnlyAllowlistedMethodsCanBeSent(t *testing.T) {
	out := &buffer{}
	s := &session{stdin: out}
	for _, forbidden := range []string{
		"account/rateLimitResetCredit/consume",
		"account/login/start",
		"account/logout",
		"config/value/write",
		"thread/start",
		"",
	} {
		if err := s.send(message{Method: forbidden}); !errors.Is(err, errForbiddenMethod) {
			t.Fatalf("send(%q) = %v, want errForbiddenMethod", forbidden, err)
		}
	}
	if out.Len() != 0 {
		t.Fatalf("a forbidden method reached the server: %q", out.String())
	}
	for _, allowed := range []string{methodInitialize, methodInitialized, methodReadLimits} {
		if err := s.send(message{Method: allowed}); err != nil {
			t.Fatalf("send(%q) = %v", allowed, err)
		}
	}
	if len(allowedMethods) != 3 {
		t.Fatalf("the allowlist grew to %d methods; update SPECS §3.2 and this test", len(allowedMethods))
	}
}
