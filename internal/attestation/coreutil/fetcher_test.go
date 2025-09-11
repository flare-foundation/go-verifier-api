package coreutil

import (
	"errors"
	"testing"
	"time"
)

func TestRetry_SuccessFirstAttempt(t *testing.T) {
	want := 42
	op := func() (int, error) {
		return want, nil
	}
	got, err := Retry(3, time.Millisecond, op, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != want {
		t.Fatalf("expected %d, got %d", want, got)
	}
}

func TestRetry_SuccessAfterRetries(t *testing.T) {
	attempts := 0
	op := func() (string, error) {
		attempts++
		if attempts < 3 {
			return "", errors.New("temporary failure")
		}
		return "ok", nil
	}
	got, err := Retry(5, time.Millisecond, op, nil)
	if err != nil {
		t.Fatalf("expected success, got error %v", err)
	}
	if got != "ok" {
		t.Fatalf("expected ok, got %s", got)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetry_ExhaustRetries(t *testing.T) {
	op := func() (int, error) {
		return 0, errors.New("always fails")
	}
	_, err := Retry(3, time.Millisecond, op, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRetry_BreakOn(t *testing.T) {
	specialErr := errors.New("stop now")
	attempts := 0
	op := func() (string, error) {
		attempts++
		if attempts == 2 {
			return "bad", specialErr
		}
		return "", errors.New("regular failure")
	}
	got, err := Retry(5, time.Millisecond, op, func(e error) bool {
		return errors.Is(e, specialErr)
	})
	if err != specialErr {
		t.Fatalf("expected specialErr, got %v", err)
	}
	if got != "bad" {
		t.Fatalf("expected bad, got %s", got)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestRetry_ReturnsLastResult(t *testing.T) {
	op := func() (int, error) {
		return 99, errors.New("fail but keep result")
	}
	got, err := Retry(2, time.Millisecond, op, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got != 99 {
		t.Fatalf("expected last result 99, got %d", got)
	}
}
