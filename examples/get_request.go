package main

import (
	"context"
	"fmt"

	"github.com/SomniSom/goreq"
)

func main() {
	String()
	Byte()
	Json()
}

// String fetches data from the specified URL and returns it as a string.
// It performs a GET request to "https://httpbin.org/get".
// The returned string contains the response body from the server.
func String() {
	str, _ := goreq.New[string](context.Background(), "https://httpbin.org/get").Fetch()
	fmt.Println("String:", str)
}

// Byte fetches data from the specified URL and returns it as a byte array.
// It performs a GET request to "https://httpbin.org/get".
// The returned byte array represents the response body from the server.
// The byte array is then converted to a string for printing.
func Byte() {
	bt, _ := goreq.New[[]byte](context.Background(), "https://httpbin.org/get").Fetch()
	fmt.Println("Byte as string:", string(bt))
}

// HttpBinGetResponse is a struct used to parse JSON responses from httpbin.org.
type HttpBinGetResponse struct {
	Args struct {
	} `json:"args"`
	Headers map[string]string `json:"headers"`
	Origin  string            `json:"origin"`
	Url     string            `json:"url"`
}

// Json fetches data from the specified URL and parses the response as a JSON object.
// It performs a GET request to "https://httpbin.org/get" and unmarshals the response body
// into a HttpBinGetResponse struct.  The resulting struct is then printed to the console.
func Json() {
	resp, _ := goreq.New[HttpBinGetResponse](context.Background(), "https://httpbin.org/get").Fetch()
	fmt.Println("From json struct:", resp)
}
