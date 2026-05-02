package errors

import (
	"testing"

	hs "github.com/amarbel-llc/purse-first/libs/dewey/0/http_statuses"
)

type testErrDisamb struct{}

func TestBadRequestf(t *testing.T) {
	badRequest := BadRequestf("testing")
	badRequest = Wrap(badRequest)

	if !Is400BadRequest(badRequest) {
		t.Errorf("expected bad request")
	}
}

func TestBadRequest(t *testing.T) {
	badRequest := BadRequest(NewWithType[testErrDisamb]("testing"))
	badRequest = Wrap(badRequest)

	if !Is400BadRequest(badRequest) {
		t.Errorf("expected bad request")
	}
}

func TestErr422MatchesCode422(t *testing.T) {
	err := Err422UnprocessableEntity.Errorf("tampered")

	if !IsHTTPError(err, hs.Code422UnprocessableEntity) {
		t.Errorf("Err422UnprocessableEntity should match Code422UnprocessableEntity")
	}

	if IsHTTPError(err, hs.Code409Conflict) {
		t.Errorf("Err422UnprocessableEntity should not match Code409Conflict")
	}
}

func TestErr409MatchesCode409(t *testing.T) {
	err := Err409Conflict.Errorf("conflict")

	if !IsHTTPError(err, hs.Code409Conflict) {
		t.Errorf("Err409Conflict should match Code409Conflict")
	}

	if IsHTTPError(err, hs.Code422UnprocessableEntity) {
		t.Errorf("Err409Conflict should not match Code422UnprocessableEntity")
	}
}
