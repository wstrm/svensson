package giphy

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"time"
)

var httpClient *http.Client

func init() {
	// Configure the timeouts of the HTTP client to be rather short.
	httpTransport := &http.Transport{
		MaxIdleConns:    1,
		IdleConnTimeout: 5 * time.Second,
	}

	httpClient = &http.Client{Transport: httpTransport}
}

type GiphyImage struct {
	URL string `json:"url"`
}

type GiphySearchResult struct {
	Images map[string]GiphyImage `json:"images"`
}

type GiphySearchResults []GiphySearchResult

type GiphyResponse map[string]*json.RawMessage

// FindGif uses Giphy's API to find matching Gif's from the provided query.
func FindGif(key, query string) (uri string, err error) {
	req, err := url.Parse("https://api.giphy.com/v1/gifs/search")
	if err != nil {
		log.Fatal(err)
	}

	q := req.Query()

	q.Set("api_key", key)
	q.Set("q", query)
	q.Set("limit", "1")
	q.Set("offset", "0")
	q.Set("rating", "PG-13")
	q.Set("lang", "en")

	req.RawQuery = q.Encode()

	res, err := httpClient.Get(req.String())
	if err != nil {
		return
	}

	var g GiphyResponse
	err = json.NewDecoder(res.Body).Decode(&g)
	if err != nil {
		return
	}

	var (
		data *json.RawMessage
		ok   bool
	)
	if data, ok = g["data"]; !ok {
		err = errors.New("missing data in Giphy response")
		return
	}

	var s GiphySearchResults
	err = json.Unmarshal(*data, &s)
	if err != nil {
		return
	}

	if len(s) == 0 {
		err = errors.New("empty search result from Giphy")
		return
	}

	images := s[0].Images
	originalImage, ok := images["downsized"]
	if !ok {
		err = errors.New("missing downsized image from Giphy images result")
		return
	}

	url, err := url.Parse(originalImage.URL)
	if err != nil {
		return
	}

	url.RawQuery = "" // Remove tracking from URL.
	uri = url.String()

	return
}
