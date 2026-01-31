package dbconnpool

import (
	"testing"
)

func TestConfig(t *testing.T) {
	opt := &Options{
		Host: "",
		Port: "",
	}

	opt.init()

	if opt.Host != "localhost" {
		t.Errorf("got %s, wanted %s", opt.Host, "localhost")
	}

	if opt.Port != "5432" {
		t.Errorf("got %s, wanted %s", opt.Port, opt.Host)
	}
}
