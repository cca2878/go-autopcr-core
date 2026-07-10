package remote

import (
	"context"
	"errors"
	"testing"

	"github.com/cca2878/gtrv-go"
)

// fakeValidator 是不触网的 gtrv.Validator 打桩。
type fakeValidator struct {
	res *gtrv.ValidationResult
	err error
}

func (f fakeValidator) Validate(ctx context.Context) (*gtrv.ValidationResult, error) {
	return f.res, f.err
}

func TestSolveMapsResultAndSeccode(t *testing.T) {
	fv := fakeValidator{res: &gtrv.ValidationResult{
		Challenge: "chal", Gt: "gt", GtUserId: "guid", Validate: "vali",
	}}
	s := Wrap(fv)

	got, err := s.Solve(context.Background())
	if err != nil {
		t.Fatalf("Solve 返回错误: %v", err)
	}
	if got.Challenge != "chal" {
		t.Errorf("Challenge=%q, 期望 chal", got.Challenge)
	}
	if got.Validate != "vali" {
		t.Errorf("Validate=%q, 期望 vali", got.Validate)
	}
	if got.Seccode != "vali|jordan" {
		t.Errorf("Seccode=%q, 期望 vali|jordan", got.Seccode)
	}
}

func TestSolvePropagatesError(t *testing.T) {
	sentinel := errors.New("boom")
	s := Wrap(fakeValidator{err: sentinel})
	if _, err := s.Solve(context.Background()); !errors.Is(err, sentinel) {
		t.Errorf("期望透出底层错误，得到 %v", err)
	}
}

func TestValidatorReturnsUnderlying(t *testing.T) {
	fv := fakeValidator{res: &gtrv.ValidationResult{}}
	s := Wrap(fv)
	if s.Validator() != gtrv.Validator(fv) {
		t.Error("Validator() 未返回底层同一实例")
	}
}
