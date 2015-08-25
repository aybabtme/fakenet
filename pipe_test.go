package fakenet

import (
	"io/ioutil"
	"testing"
	"time"

	"golang.org/x/net/context"
)

func TestDuplex(t *testing.T) {
	defer time.AfterFunc(time.Second, func() { panic("too long") }).Stop()
	wantReq := "hello server!"
	wantResp := "hello client!"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, server := newDuplex(ctx)

	go func() {

		req, err := ioutil.ReadAll(server)
		if err != nil {
			panic(err)
		}
		if want, got := wantReq, string(req); want != got {
			t.Errorf("want client request %q, got %q", want, got)
		}

		_, err = server.Write([]byte(wantResp))
		if err != nil {
			panic(err)
		}
		server.Close()
	}()

	_, err := client.Write([]byte(wantReq))
	if err != nil {
		panic(err)
	}
	client.Close()

	resp, err := ioutil.ReadAll(client)
	if err != nil {
		panic(err)
	}
	if want, got := wantResp, string(resp); want != got {
		t.Errorf("want server response %q, got %q", want, got)
	}
	t.Logf("want=%q", wantResp)
	t.Logf(" got=%q", string(resp))

}
