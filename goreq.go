package goreq

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
	"strconv"
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

// Client set http client
func (r *request[T]) Client(cl *http.Client) *request[T] {
	r.client = cl
	return r
}

// Path set path url
func (r *request[T]) Path(path string) *request[T] {
	r.u.Path = path
	return r
}

func (r *request[T]) Params(attr ...string) *request[T] {
	q := r.u.Query()
	for len(attr) > 0 {
		if len(attr) > 1 {
			q.Add(attr[0], attr[1])
			attr = attr[2:]
		} else {
			q.Add(attr[0], "")
			attr = attr[0:]
		}
	}
	r.u.RawQuery = q.Encode()
	return r
}

// Header add header Deprecated
func (r *request[T]) Header(key, value string) *request[T] {
	r.headers[key] = append(r.headers[key], value)
	return r
}

// Headers add headers key/value
func (r *request[T]) Headers(attr ...string) *request[T] {
	for len(attr) > 0 {
		if len(attr) > 1 {
			r.headers[attr[0]] = append(r.headers[attr[0]], attr[1])
			attr = attr[2:]
		} else {
			r.headers[attr[0]] = append(r.headers[attr[0]], "")
			attr = nil
		}
	}
	return r
}

// Method http method
func (r *request[T]) Method(method string) *request[T] {
	if !slices.Contains([]string{http.MethodGet, http.MethodPost, http.MethodConnect, http.MethodDelete,
		http.MethodOptions, http.MethodPatch, http.MethodTrace, http.MethodHead}, r.method) {
		r.err = errors.New("method incorrect")
	}
	r.method = method
	return r
}

// BodyJson set object on marshal to json
func (r *request[T]) BodyJson(dt any) *request[T] {
	r.body, r.err = json.Marshal(dt)
	return r
}

// BodyRaw set body slice byte
func (r *request[T]) BodyRaw(raw []byte) *request[T] {
	r.body = raw
	return r
}

// Proxy is not work
func (r *request[T]) Proxy(proxy string) *request[T] {
	r.proxy = proxy
	return r
}

// IsJson if T is struct and response is application/json use this for set unmarshal
func (r *request[T]) IsJson() *request[T] {
	r.isJson = true
	return r
}

func (r *request[T]) any(t any, b io.Reader) error {
	dt, err := io.ReadAll(b)
	if err != nil {
		return err
	}
	switch t.(type) {
	case *[]byte:
		*t.(*[]byte) = dt
	case *string:
		*t.(*string) = string(dt)
	case int:
		*t.(*int), err = strconv.Atoi(string(dt))
		if err != nil {
			return err
		}
	default:
		t = dt
	}
	return nil
}

func (r *request[T]) Dump() ([]byte, error) {
	//region request block
	var err error
	var req *http.Request

	if len(r.body) > 0 {
		rdr := bytes.NewReader(r.body)
		req, err = http.NewRequest(r.method, r.u.String(), rdr)
	} else {
		req, err = http.NewRequest(r.method, r.u.String(), nil)
	}
	if err != nil {
		return nil, err
	}
	req.Header = r.headers
	//endregion
	return httputil.DumpRequest(req, true)
}

// Fetch fetch request
func (r *request[T]) Fetch(ctx context.Context) (T, error) {
	var t T
	if r.err != nil {
		return t, r.err
	}
	//region request block
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

	req.WithContext(ctx)
	req.Header = r.headers
	//endregion

	//region client block
	if r.client == nil {
		r.client = http.DefaultClient
	}
	proxyUrl, err := url.Parse(r.proxy)
	if t, ok := r.client.Transport.(*http.Transport); ok {
		t.Proxy = http.ProxyURL(proxyUrl)
		r.client.Transport = t
	}
	//endregion

	resp, err := r.client.Do(req)
	if err != nil {
		return t, err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if r.isJson {
		dec := json.NewDecoder(resp.Body)
		r.err = dec.Decode(&t)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			if r.err != nil {
				return t, errors.New(fmt.Sprintf("%s: status code incorrect, error decode body: %s\n", resp.Status, r.err.Error()))
			}
			return t, errors.New(fmt.Sprintf("%s: status code incorrect", resp.Status))
		}
		return t, r.err
	}
	r.err = r.any(&t, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if r.err != nil {
			return t, errors.New(fmt.Sprintf("%s: status code incorrect, error decode body: %s\n", resp.Status, r.err.Error()))
		}
		return t, errors.New(fmt.Sprintf("%s: status code incorrect", resp.Status))
	}
	return t, err
}

// New create new request
// T any - string or byte or struct if IsJson
func New[T any](link string) *request[T] {
	r := new(request[T])
	r.method = http.MethodGet
	r.headers = map[string][]string{}
	r.u, r.err = url.Parse(link)
	if r.err != nil {
		r.u = new(url.URL)
	}
	return r
}
