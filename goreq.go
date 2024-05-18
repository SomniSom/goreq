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
	"time"
)

type request[T any] struct {
	u                 *url.URL
	link              string
	method            string
	headers           map[string][]string
	data              []byte
	dataReader        *io.Reader
	client            *http.Client
	proxy             string
	body              []byte
	result            T
	isJson            bool
	retBody           *bytes.Buffer
	finalReq          *http.Request
	cntRepeat         uint
	timeouts          []time.Duration
	repeatStatusCodes []int
	repeatHttpErrors  []error
	err               error
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
	if len(attr)%2 != 0 {
		r.err = errors.New("incompatible query parameter")
		return r
	}
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
	r.Headers("Content-Type", "application/json")
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
	if proxy != "" {
		proxyUrl, err := url.Parse(r.proxy)
		if err != nil {
			r.err = err
			return r
		}
		if r.client.Transport == nil {
			r.client.Transport = &http.Transport{
				Proxy: http.ProxyURL(proxyUrl),
			}
			return r
		}
		if t, ok := r.client.Transport.(*http.Transport); ok {
			t.Proxy = http.ProxyURL(proxyUrl)
			r.client.Transport = t
		}
	}
	return r
}

func (r *request[T]) ToBody(b *bytes.Buffer) *request[T] {
	if b == nil {
		return r
	}
	r.retBody = b
	return r
}

func (r *request[T]) any(t any, dt []byte) error {
	var err error
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
func (r *request[T]) checkType(t any) {
	switch t.(type) {
	case *[]byte:
		r.isJson = false
	case []byte:
		r.isJson = false
	case *string:
		r.isJson = false
	case string:
		r.isJson = false
	case int:
		r.isJson = false
	case *int:
		r.isJson = false
	default:
		r.isJson = true
	}
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

func (r *request[T]) request() (*http.Response, error) {
	resp, err := r.client.Do(r.finalReq)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (r *request[T]) inStatusCode(statusCode int) bool {
	if len(r.repeatStatusCodes) == 0 {
		return true
	}
	for _, code := range r.repeatStatusCodes {
		if statusCode == code {
			return true
		}
	}
	return false
}

func (r *request[T]) inHttpError(err error) bool {
	if len(r.repeatHttpErrors) == 0 {
		return true
	}
	for _, httpErr := range r.repeatHttpErrors {
		if errors.Is(err, httpErr) {
			return true
		}
	}
	return false
}

func (r *request[T]) RepeatParams(statusCode []int, httpError []error) *request[T] {
	r.repeatStatusCodes = statusCode
	r.repeatHttpErrors = httpError
	return r
}

func (r *request[T]) Repeat(rp uint, timeouts ...time.Duration) *request[T] {
	r.cntRepeat = rp
	r.timeouts = timeouts
	return r
}

// Fetch fetch request
func (r *request[T]) Fetch(ctx context.Context) (T, error) {
	var t T
	if r.err != nil {
		return t, r.err
	}
	//region request block
	var err error

	if len(r.body) > 0 {
		rdr := bytes.NewReader(r.body)
		r.finalReq, err = http.NewRequest(r.method, r.u.String(), rdr)
	} else {
		r.finalReq, err = http.NewRequest(r.method, r.u.String(), nil)
	}
	if err != nil {
		return t, err
	}
	//r.finalReq.WithContext(ctx)

	//fix replace default headers
	for k, v := range r.headers {
		r.finalReq.Header[k] = v
	}
	//endregion

	var cnt uint
	//_ = cnt
	var resp *http.Response
	//resp, err = r.request()
	for resp, err = r.request(); (err != nil || resp.StatusCode > 200) && cnt < r.cntRepeat; {
		if cnt+1 >= r.cntRepeat {
			break
		}
		if err != nil {
			if !r.inHttpError(err) {
				break
			}
		}
		if resp != nil {
			if !r.inStatusCode(resp.StatusCode) {
				break
			}
		}
		if len(r.timeouts) > 0 {
			if len(r.timeouts) > int(cnt) {
				time.Sleep(r.timeouts[cnt])
			} else {
				time.Sleep(r.timeouts[len(r.timeouts)-1])
			}
		}
		cnt++
	}
	if err != nil {
		return t, err
	}
	defer func(Body io.ReadCloser) {
		if Body != nil {
			_ = Body.Close()
		}
	}(resp.Body)

	var rdr io.Reader
	if r.retBody != nil {
		rdr = io.TeeReader(resp.Body, r.retBody)
	} else {
		rdr = resp.Body
	}

	if r.isJson {
		dec := json.NewDecoder(rdr)
		r.err = dec.Decode(&t)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			if r.err != nil {
				return t, errors.New(fmt.Sprintf("%s: status code incorrect, error decode body: %s\n", resp.Status, r.err.Error()))
			}
			return t, errors.New(fmt.Sprintf("%s: status code incorrect", resp.Status))
		}
		return t, r.err
	}
	dt, err := io.ReadAll(rdr)
	if err != nil {
		return t, err
	}
	r.err = r.any(&t, dt)
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
	var t T
	r.checkType(t)
	r.method = http.MethodGet
	r.headers = map[string][]string{}
	r.client = http.DefaultClient
	r.u, r.err = url.Parse(link)
	if r.err != nil {
		r.u = new(url.URL)
	}
	return r
}

//todo repeat N with timeout, exponentional time.
