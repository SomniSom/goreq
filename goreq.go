package goreq

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

//region Multipart

type Multipart struct {
	Ctx  context.Context
	wr   *multipart.Writer
	body *bytes.Buffer
}

func (m *Multipart) AddFile(fieldName string, filename string) *Multipart {
	m.initBody()
	dt, err := os.ReadFile(filename)
	if err != nil {
		m.contextErr(err)
		return m
	}
	return m.AddFileData(fieldName, filename, dt)
}

func (m *Multipart) AddFileData(fieldName string, filename string, data []byte) *Multipart {
	m.initBody()
	wr, err := m.wr.CreateFormFile(fieldName, filename)
	if err != nil {
		m.contextErr(err)
		return m
	}
	_, err = wr.Write(data)
	if err != nil {
		m.contextErr(err)
		return m
	}

	return m
}
func (m *Multipart) Param(fieldName string, s string) *Multipart {
	m.initBody()
	return m.contextErr(m.wr.WriteField(fieldName, s))
}

func (m *Multipart) initBody() {
	if m.body == nil && m.wr == nil {
		m.body = new(bytes.Buffer)
		m.wr = multipart.NewWriter(m.body)
	}
	if m.Ctx == nil {
		m.Ctx = context.Background()
	}
}

func (m *Multipart) contextErr(err error) *Multipart {
	if err == nil {
		return m
	}
	ctx, cancel := context.WithCancelCause(m.Ctx)
	m.Ctx = ctx
	cancel(err)
	return m
}

func (m *Multipart) make() (string, []byte) {
	defer func() {
		m.body.Reset()
	}()

	err := m.wr.Close()
	if err != nil {
		m.contextErr(err)
		return "", nil
	}
	return m.wr.FormDataContentType(), m.body.Bytes()
}

//endregion

//region Retry

// RetryOptions is params for retry Request
type RetryOptions interface {
	Repeat(response *http.Response, err error) bool
	Sleep(counter int) bool
}

type DefaultRetryOptions struct {
	Count           int
	HttpErrors      []error
	HttpStatusCodes []int
}

func (d DefaultRetryOptions) inStatusCode(statusCode int) bool {
	if len(d.HttpStatusCodes) == 0 {
		return true
	}
	for _, code := range d.HttpStatusCodes {
		if statusCode == code {
			return true
		}
	}
	return false
}

func (d DefaultRetryOptions) inHttpError(err error) bool {
	if err == nil {
		return false
	}
	if len(d.HttpErrors) == 0 {
		return true
	}
	for _, httpErr := range d.HttpErrors {
		if errors.Is(err, httpErr) {
			return true
		}
	}
	return false
}

func (d DefaultRetryOptions) Repeat(response *http.Response, err error) bool {
	if response == nil {
		return d.inHttpError(err)
	}
	slog.Debug("Retry", "response", response.StatusCode, "err", err)

	return d.inHttpError(err) && d.inStatusCode(response.StatusCode)
}

func (d DefaultRetryOptions) Sleep(counter int) bool {
	if counter > d.Count {
		return false
	}
	slog.Debug("Sleep", "counter", counter)
	time.Sleep(time.Duration(counter) * time.Second)
	return true
}

type emptyOptions struct{}

func (e emptyOptions) Repeat(_ *http.Response, _ error) bool {
	return false
}

func (e emptyOptions) Sleep(_ int) bool {
	panic("not implemented")
}

//endregion

type Request[T any] struct {
	ctx               context.Context
	u                 *url.URL
	link              string
	method            string
	headers           map[string][]string
	data              []byte
	dataReader        *io.Reader
	client            *http.Client
	proxy             string
	body              []byte
	multipart         *Multipart
	result            T
	isJson            bool
	retBody           *bytes.Buffer
	finalReq          *http.Request
	retryOptions      RetryOptions
	timeouts          []time.Duration
	repeatStatusCodes []int
	repeatHttpErrors  []error
	err               error
	lastResponse      *http.Response
	cookie            []*http.Cookie
}

// Clone this object
func (r *Request[T]) Clone() *Request[T] {
	rq := &Request[T]{}
	*rq = *r
	if r.finalReq != nil {
		*(rq.finalReq) = *(r.finalReq)
	} else {
		rq.finalReq = new(http.Request)
	}

	if r.retBody != nil {
		*(rq.retBody) = *(r.retBody)
	}

	if r.client != nil {
		*(rq.client) = *(r.client)
	}
	if r.data != nil {
		*(rq.dataReader) = *(r.dataReader)
	}
	if r.lastResponse != nil {
		*(rq.lastResponse) = *(r.lastResponse)
	}
	return rq
}

// Client set http client
func (r *Request[T]) Client(cl *http.Client) *Request[T] {
	if r.ctx.Err() != nil {
		return r
	}
	r.client = cl
	return r
}

// Path set path url
func (r *Request[T]) Path(path string) *Request[T] {
	if r.ctx.Err() != nil {
		return r
	}
	r.u.Path = path
	return r
}

// Cookie This method, is designed to add or update the cookies associated with a Request object.
func (r *Request[T]) Cookie(c []*http.Cookie) *Request[T] {
	r.cookie = c
	return r
}

// Params The method adds query parameters to the request URL. It accepts a variadic number of string arguments, where
// each pair of arguments represents a key-value pair for the query parameters. `Params`
// example: request.Params("key1", "value1", "key2", "value2")
func (r *Request[T]) Params(attr ...string) *Request[T] {
	if r.ctx.Err() != nil {
		return r
	}
	q := r.u.Query()
	if len(attr)%2 != 0 {
		r.err = errors.New("incompatible query parameter")
		r.contextErr(r.err)
		return r
	}
	for len(attr) > 0 {
		if len(attr) > 1 {
			if q.Get(attr[0]) != "" {
				q.Set(attr[0], attr[1])
			} else {
				q.Add(attr[0], attr[1])
			}
			attr = attr[2:]
		} else {
			q.Add(attr[0], "")
			attr = attr[0:]
		}
	}
	r.u.RawQuery = q.Encode()
	return r
}

// Headers add headers key/value
func (r *Request[T]) Headers(attr ...string) *Request[T] {
	if r.ctx.Err() != nil {
		return r
	}
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
func (r *Request[T]) Method(method string) *Request[T] {
	if r.ctx.Err() != nil {
		return r
	}
	if !slices.Contains([]string{http.MethodGet, http.MethodPost, http.MethodConnect, http.MethodDelete,
		http.MethodOptions, http.MethodPatch, http.MethodTrace, http.MethodHead, http.MethodPut}, r.method) {
		r.err = errors.New("method incorrect")
		r.contextErr(r.err)
	}
	r.method = method
	return r
}

// BodyJson set object on marshal to JSON
func (r *Request[T]) BodyJson(dt any) *Request[T] {
	if r.ctx.Err() != nil {
		return r
	}
	r.body, r.err = json.Marshal(dt)
	r.contextErr(r.err)
	r.Headers("Content-Type", "application/json")
	r.method = http.MethodPost
	return r
}

// BodyMultipart add multipart form data
func (r *Request[T]) BodyMultipart(m *Multipart) *Request[T] {
	if r.ctx.Err() != nil {
		return r
	}
	r.method = http.MethodPost
	r.multipart = m
	return r
}

// BodyRaw set body slice byte
func (r *Request[T]) BodyRaw(raw []byte) *Request[T] {
	if r.ctx.Err() != nil {
		return r
	}
	r.method = http.MethodPost
	r.body = raw
	return r
}

// Proxy is not work
func (r *Request[T]) Proxy(proxy string) *Request[T] {
	if r.ctx.Err() != nil {
		return r
	}
	r.proxy = strings.TrimSpace(proxy)
	if r.proxy != "" {
		proxyUrl, err := url.Parse(r.proxy)
		if err != nil {
			r.contextErr(err)
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

func (r *Request[T]) ToBody(b *bytes.Buffer) *Request[T] {
	if r.ctx.Err() != nil {
		return r
	}
	if b == nil {
		return r
	}
	r.retBody = b
	return r
}

func (r *Request[T]) any(t any, dt []byte) error {
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
	case bool:
		*t.(*bool), err = strconv.ParseBool(string(dt))
		if err != nil {
			return err
		}
	case *bool:
		*t.(*bool), err = strconv.ParseBool(string(dt))
		if err != nil {
			return err
		}
	default:
		t = dt
	}
	return nil
}
func (r *Request[T]) checkType(t any) {
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

// Dump return raw http request as text
func (r *Request[T]) Dump() ([]byte, error) {
	if r.ctx.Err() != nil {
		return nil, r.ctx.Err()
	}
	//region Request block
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

func (r *Request[T]) request() (*http.Response, error) {
	resp, err := r.client.Do(r.finalReq)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// Retry functional mechanism to perform actions repetitively until successful.
func (r *Request[T]) Retry(options RetryOptions) *Request[T] {
	if r.ctx.Err() != nil {
		return r
	}
	r.retryOptions = options
	return r
}

// GetLastResponse get *http.Response of last request, but body is empty and closed, use method ToBody
func (r *Request[T]) GetLastResponse() *http.Response {
	return r.lastResponse
}

func (r *Request[T]) makeRequest() error {
	//region Request block
	if len(r.body) > 0 {
		//Fix http method
		if r.finalReq.Method == http.MethodGet {
			r.finalReq.Method = http.MethodPost
		}
		rdr := bytes.NewReader(r.body)
		r.finalReq, r.err = http.NewRequest(r.method, r.u.String(), rdr)
	} else if r.multipart != nil {
		if r.finalReq.Method == http.MethodGet {
			r.finalReq.Method = http.MethodPost
		}
		//Multipart body
		ctt, data := r.multipart.make()
		if r.multipart.Ctx.Err() != nil {
			r.contextErr(r.multipart.Ctx.Err())
			return r.multipart.Ctx.Err()
		}
		rdr := bytes.NewReader(data)
		r.finalReq, r.err = http.NewRequest(r.method, r.u.String(), rdr)
		r.finalReq.Header["Content-Type"] = []string{ctt}
	} else {
		r.finalReq, r.err = http.NewRequest(r.method, r.u.String(), nil)
	}

	if r.err != nil {
		r.contextErr(r.err)
		return r.err
	}

	//Set cookies
	if len(r.cookie) > 0 {
		for _, v := range r.cookie {
			r.finalReq.AddCookie(v)
		}
	}

	//fix replace default headers
	for k, v := range r.headers {
		r.finalReq.Header[k] = v
	}
	//endregion
	return nil
}

// Fetch fetch Request
func (r *Request[T]) Fetch() (T, error) {
	var t T
	if r.err != nil {
		return t, r.err
	}
	if r.ctx.Err() != nil {
		return t, r.ctx.Err()
	}

	if err := r.makeRequest(); err != nil {
		r.contextErr(err)
		return t, err
	}

	var cnt = 1
	var resp *http.Response

	//Retry method
	for resp, r.err = r.request(); r.retryOptions.Repeat(resp, r.err); {
		if r.err != nil || resp == nil {
			r.contextErr(r.err)
			return t, r.err
		}

		if !r.retryOptions.Sleep(cnt) {
			break
		}
		slog.Debug("Retry counter", "cnt", cnt, "status", resp.Status, "error", r.err)
		cnt++
	}
	if resp == nil {
		return t, r.err
	}
	r.lastResponse = resp
	if r.err != nil {
		r.contextErr(r.err)
		return t, r.err
	}

	defer func(Body io.ReadCloser) {
		if Body != nil {
			_ = Body.Close()
		}
	}(resp.Body)

	//region compress check
	var reader io.Reader
	//gzip, deflate, br, zstd
	switch resp.Header.Get("Content-Encoding") {
	case "zstd":
		reader, r.err = zstd.NewReader(resp.Body)
		if r.err != nil {
			r.contextErr(r.err)
			return t, r.err
		}
	case "br":
		reader = brotli.NewReader(resp.Body)
	case "gzip":
		reader, r.err = gzip.NewReader(resp.Body)
		if r.err != nil {
			r.contextErr(r.err)
			return t, r.err
		}
	default:
		reader = resp.Body
	}
	//endregion

	//region check use method ToBody
	var rdr io.Reader
	if r.retBody != nil {
		rdr = io.TeeReader(reader, r.retBody)
	} else {
		rdr = reader
	}
	//endregion

	//region cookie
	r.cookie = r.cookie[:0]
	for _, cookie := range resp.Cookies() {
		r.cookie = append(r.cookie, cookie)
	}
	//endregion

	if r.isJson {
		dec := json.NewDecoder(rdr)
		r.err = dec.Decode(&t)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			if r.err != nil {
				r.err = errors.New(fmt.Sprintf("%s: status code incorrect, error decode body: %s\n", resp.Status, r.err.Error()))
				r.contextErr(r.err)
				return t, r.err
			}
			r.err = errors.New(fmt.Sprintf("%s: status code incorrect", resp.Status))
			r.contextErr(r.err)
			return t, r.err
		}
		return t, r.err
	}

	var dt []byte
	dt, r.err = io.ReadAll(rdr)
	if r.err != nil {
		r.contextErr(r.err)
		return t, r.err
	}

	r.err = r.any(&t, dt)
	if r.err != nil {
		r.contextErr(r.err)
		return t, r.err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if r.err != nil {
			r.err = errors.New(fmt.Sprintf("%s: status code incorrect, error decode body: %s\n", resp.Status, r.err.Error()))
			r.contextErr(r.err)
			return t, r.err
		}
		r.err = errors.New(fmt.Sprintf("%s: status code incorrect", resp.Status))
		r.contextErr(r.err)
		return t, r.err
	}
	return t, r.err
}

func (r *Request[T]) contextErr(err error) *Request[T] {
	if err == nil {
		return r
	}
	ctx, cancel := context.WithCancelCause(r.ctx)
	r.ctx = ctx
	cancel(err)
	return r
}

// New create new Request
// T any - string or byte or struct if IsJson
func New[T any](ctx context.Context, link string) *Request[T] {
	var t T

	r := new(Request[T])
	r.ctx = ctx
	r.checkType(t)

	//set default method
	r.method = http.MethodGet

	//set default headers
	r.headers = map[string][]string{
		"Accept-Encoding": {"deflate,gzip,zstd,br"},
		"User-Agent":      {"SomniSom-goreq/1.0"},
	}
	r.client = http.DefaultClient
	r.retryOptions = emptyOptions{}
	r.u, r.err = url.Parse(link)
	if r.err != nil {
		r.contextErr(r.err)
	}
	return r
}
