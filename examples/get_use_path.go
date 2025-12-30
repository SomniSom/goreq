package main

import (
	"context"
	"fmt"
	"github.com/SomniSom/goreq"
)

var req = goreq.New[string](context.Background(), "https://httpbin.org").Headers("X-Test", "test=req")

func main() {
	Get()
	PostBodyRaw()
	PostBodyJson()
	PostBodyMultipart()
}

// Get performs a GET request to the "/get" endpoint of the base URL.
// It clones the 'req' object to create a new request context.
// The response body is fetched and printed to the console.
func Get() {
	r := req.Clone()
	str, err := r.Path("/get").Fetch()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Get:", str)
}

// PostBodyRaw performs a POST request to the "/post" endpoint with a raw body.
// It clones the 'req' object and sends a POST request with a raw body containing the string "test".
// The Content-Type header is set to "text/plain", and the response body is printed.
// The clone of 'req' is created to isolate the request's configuration.
func PostBodyRaw() {
	r := req.Clone()
	bt, err := r.Path("/post").BodyRaw([]byte("test")).Headers("Content-Type", "text/plain").Fetch()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Post:", bt)
}

// PostBodyJson performs a POST request to the "/post" endpoint with a JSON body.
// It clones the 'req' object and sends a POST request with a JSON body.
// The Content-Type header is automatically set to "application/json" by the goreq library.
// The response body is printed.  The clone of 'req' is created to isolate the request's configuration.
func PostBodyJson() {
	r := req.Clone()
	// Content-type auto set "application/json"
	bt, err := r.Path("/post").BodyJson(struct {
		Test string `json:"test"`
	}{
		Test: "test",
	}).Fetch()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Post:", bt)
}

// PostBodyMultipart performs a POST request to the "/post" endpoint with a multipart body.
// It clones the 'req' object and sends a POST request with a multipart body that includes a file named "file"
// It uses "go.mod" as the file content.  The clone of 'req' is created to isolate the request's configuration.
func PostBodyMultipart() {
	r := req.Clone()
	m := &goreq.Multipart{Ctx: context.Background()}
	m.AddFile("file", "go.mod")
	bt, err := r.Path("/post").BodyMultipart(m).Fetch()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Post:", bt)
}
