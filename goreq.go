package goreq

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"reflect"
)

type request[T any] struct {
	u          *url.URL
	link       string
	method     string
	headers    map[string][]string
	data       []byte
	dataReader *io.Reader
	client     *http.Client
	proxy      string
	body       []byte
	result     T
	isJson     bool
	err        error
}

func (r *request[T]) Client(cl *http.Client) *request[T] {
	r.client = cl
	return r
}

func (r *request[T]) Path(path string) *request[T] {
	r.u.Path = path
	return r
}

func (r *request[T]) Header(key, value string) *request[T] {
	r.headers[key] = append(r.headers[key], value)
	return r
}

func (r *request[T]) Method(method string) *request[T] {
	r.method = method
	return r
}

func (r *request[T]) BodyJson(dt any) *request[T] {
	r.body, r.err = json.Marshal(dt)
	return r
}

func (r *request[T]) BodyRaw(raw []byte) *request[T] {
	r.body = raw
	return r
}

func (r *request[T]) Proxy(proxy string) *request[T] {
	r.proxy = proxy
	return r
}

func (r *request[T]) IsJson() *request[T] {
	r.isJson = true
	return r
}

func (r *request[T]) any(t any, b io.Reader) error {
	log.Println(reflect.TypeOf(t))
	dt, err := io.ReadAll(b)
	if err != nil {
		return err
	}
	switch t.(type) {
	case *[]byte:
		log.Println("bytes", t)
		*t.(*[]byte) = dt
	case *string:
		log.Println("string", t)
		*t.(*string) = string(dt)
	default:
		t = dt
	}
	return nil
}

func (r *request[T]) Fetch(ctx context.Context) (T, error) {
	var t T
	if r.err != nil {
		return t, r.err
	}
	var err error
	var req *http.Request

	if len(r.body) > 0 {
		rdr := bytes.NewReader(r.body)
		req, err = http.NewRequest(r.method, r.u.String(), rdr)
	} else {
		req, err = http.NewRequest(r.method, r.u.String(), nil)
	}

	if err != nil {
		return t, err
	}
	if r.client == nil {
		r.client = http.DefaultClient
	}
	req.WithContext(ctx)
	resp, err := r.client.Do(req)
	if err != nil {
		return t, err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if r.isJson {
		dec := json.NewDecoder(resp.Body)
		return t, dec.Decode(&t)
	}
	return t, r.any(&t, resp.Body)
}

func New[T any](link string) *request[T] {
	r := new(request[T])
	r.headers = map[string][]string{}
	r.u, r.err = url.Parse(link)
	return r
}
