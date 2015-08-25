package fakenet

import (
	"fmt"
	"golang.org/x/net/context"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

func ExampleListener() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	l, dial := Listener(ctx)
	defer l.Close()
	go func() {
		handler := func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			w.Write([]byte("hello world"))
		}
		err := (&http.Server{
			Handler: http.HandlerFunc(handler),
		}).Serve(l)
		if err != nil {
			fmt.Println(err)
		}
	}()

	client := http.Client{Transport: &http.Transport{
		Dial: func(_, _ string) (net.Conn, error) {
			return dial(), nil
		},
	}}

	resp, err := client.Get("http://" + l.Addr().String())
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println(resp.Status)
	fmt.Println(string(body))
	// Output:
	// 200 OK
	// hello world
}
