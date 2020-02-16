package tftp

import "testing"

func TestServer(t *testing.T) {
	svr := Server{}
	if err := svr.Init(); err != nil {
		t.Error(err)
	}
	if err := svr.DeInit(); err != nil {
		t.Error(err)
	}
}
