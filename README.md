# package `fakenet`

Experimental.

Implements a `net.Listener` that hands over `net.Conn`, which are all
handled in memory and don't actually use OS resources.

# Usage

A very bare example with a server of some sort:

```go
func server(l net.Listener) {
    for {
        conn, err := l.Accept()
        if err != nil {
            panic(err)
        }
        go serve(conn)
    }
}
```

And a client of some sort:

```go
func client(dialer func() net.Conn) {
    conn := dialer()
    doRequest(conn)
}
```

you then just create a fake listener:

```go
l, dial := Listener(ctx)
defer l.Close()
go server(l)
client(dial)
```

Or a concrete example:

```go
l, dial := Listener(ctx)
defer l.Close()

go (&http.Server{Handler: handler}).Serve(l)

client := http.Client{Transport: &http.Transport{
    Dial: func(_, _ string) (net.Conn, error) {
        return dial(), nil
    },
}}
```

# Why?

OS X 10.10.5 reduced the default number open file descriptors from 2560
to 256, making many of my tests in distributed systems run out of files.
To avoid having everybody increase their `ulimit` in order to run my
tests, I thought about implementing a stub network.
