package tftp

import (
	"os"
	"testing"
)

func TestLogger(t *testing.T) {
	l := Logger{}
	m, r := "main.log_", "reqs.log_"

	if err := l.Init(m, r); err != nil {
		t.Error(err)
	}

	if _, err := os.Stat(m); os.IsNotExist(err) {
		t.Error(err)
	}
	if _, err := os.Stat(r); os.IsNotExist(err) {
		t.Error(err)
	}

	if err := l.DeInit(); err != nil {
		t.Error(err)
	}

	os.Remove(m)
	os.Remove(r)

}
