# Goreq [![GoDoc](https://godoc.org/github.com/SomniSom/goreq?status.svg)](https://godoc.org/github.com/SomniSom/goreq) [![Go Report Card](https://goreportcard.com/badge/github.com/SomniSom/goreq)](https://goreportcard.com/report/github.com/SomniSom/goreq) [![Coverage Status](https://coveralls.io/repos/github/SomniSom/goreq/badge.svg)](https://coveralls.io/github/SomniSom/goreq)

## Features

- Simplifies HTTP client usage compared to net/http
- Can't forget to close response body
- Checks status codes by default
- Supports context.Context
- JSON serialization and deserialization helpers
- No third party dependencies

[//]: # (- Request recording and replaying for tests)

## Examples
### Simple GET into a string

<table>
<thead>
<tr>
<th><strong>code with net/http</strong></th>
<th><strong>code with requests</strong></th>
</tr>
</thead>
<tbody>
<tr>
<td>

```go
req, err := http.NewRequestWithContext(ctx,
	http.MethodGet, "http://example.com", nil)
if err != nil {
	// ...
}
res, err := http.DefaultClient.Do(req)
if err != nil {
	// ...
}
defer res.Body.Close()
b, err := io.ReadAll(res.Body)
if err != nil {
	// ...
}
s := string(b)
```
</td>
<td>

```go
s, err := goreq.New[string]("http://example.com").
	Fetch(context.Backgroun())
fmt.Println(s)
```

</td>
</tr>
<tr><td>11+ lines</td><td>1 line</td></tr>
</tbody>
</table>


### POST a raw body

<table>
<thead>
<tr>
<th><strong>code with net/http</strong></th>
<th><strong>code with requests</strong></th>
</tr>
</thead>
<tbody>
<tr>
<td>

```go
body := bytes.NewReader(([]byte(`hello, world`))
req, err := http.NewRequestWithContext(ctx, http.MethodPost,
	"https://postman-echo.com/post", body)
if err != nil {
	// ...
}
req.Header.Set("Content-Type", "text/plain")
res, err := http.DefaultClient.Do(req)
if err != nil {
	// ...
}
defer res.Body.Close()
_, err := io.ReadAll(res.Body)
if err != nil {
	// ...
}
```

</td>
<td>

```go
s, err := goreq.New[string]("https://postman-echo.com/post").
	BodyRaw([]byte(`hello, world`)).
    Header("Content-Type", "text/plain").
	Fetch(ctx)
```

</td>
</tr>
<tr><td>12+ lines</td><td>4 lines</td></tr></tbody></table>

### GET a JSON object

<table>
<thead>
<tr>
<th><strong>code with net/http</strong></th>
<th><strong>code with requests</strong></th>
</tr>
</thead>
<tbody>
<tr>
<td>

```go
var post placeholder
u, err := url.Parse("https://jsonplaceholder.typicode.com")
if err != nil {
	// ...
}
u.Path = fmt.Sprintf("/posts/%d", 1)
req, err := http.NewRequestWithContext(ctx,
	http.MethodGet, u.String(), nil)
if err != nil {
	// ...
}
res, err := http.DefaultClient.Do(req)
if err != nil {
	// ...
}
defer res.Body.Close()
b, err := io.ReadAll(res.Body)
if err != nil {
	// ...
}
err := json.Unmarshal(b, &post)
if err != nil {
	// ...
}
```
</td><td>

```go
post, err := goreq.New[placeholder]("https://jsonplaceholder.typicode.com").IsJson().
	Path(fmt.Sprintf("/posts/%d", 1)).Fetch(ctx)
```

</td>
</tr>
<tr><td>18+ lines</td><td>2 lines</td></tr></tbody></table>

### POST a JSON object and parse the response

```go
req := placeholder{
	Title:  "foo",
	Body:   "baz",
	UserID: 1,
}
res, err := goreq.New[placeholder]("https://jsonplaceholder.typicode.com").
	Path("/posts").BodyJson(req).IsJson().Fetch(ctx)
// net/http equivalent left as an exercise for the reader
```

### Set custom headers for a request

```go
// Set headers
obj, err := goreq.New[string]("https://postman-echo.com/get").
	Headers("User-Agent", "bond/james-bond",
		"Content-Type", "secret",
		"martini", "shaken").
	Fetch(ctx)
```

### Easily manipulate URLs and query parameters

```go
s, err := goreq.New[string]("https://prod.example.com/get?a=1&b=2").
	Params("b", "3","c", "4").Fetch(ctx)
```

## FAQs
In work

[//]: # ([See wiki]&#40;https://github.com/earthboundkid/requests/wiki&#41; for more details.)
