package web

import (
	"testing"
	"time"
)

func TestCheckCSRF(t *testing.T) {
	secret := "abc"
	period := time.Millisecond * 100

	token, err := GenCSRFToken(secret, period)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(token)

	err = CheckCSRF(token, secret)
	if err != nil {
		t.Errorf("failed csrf token: %v", err)
	}
	err = CheckCSRF("bad", secret)
	if err == nil {
		t.Error("valid csrf token")
	}
	time.Sleep(period)
	err = CheckCSRF(token, secret)
	if err == nil {
		t.Error("still valid csrf token")
	}
}
