package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/SomniSom/goreq"
)

func main() {
	res, err := goreq.New[string](context.Background(), "https://httpbin.org/get").Proxy("socks5://127.0.0.1:1080").Fetch()
	if err != nil {
		slog.Error(err.Error())
		return
	}
	fmt.Println(res)
}
