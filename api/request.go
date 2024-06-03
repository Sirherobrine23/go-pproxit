package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type APIRequest struct {
	Hostname, Token string
}

// Create request and parse reponse JSON insert to struct from `InsertTo`
func (api *APIRequest) JSONRequest(Path, Method string, InsertTo any, Body io.Reader, Headers map[string]string) (*http.Response, error) {
	res, err := api.Request(Path, Method, Body, Headers)
	if err != nil {
		return res, err
	}
	defer res.Body.Close()
	err = json.NewDecoder(res.Body).Decode(InsertTo)
	return res, err
}

// Create request and return http response
func (api *APIRequest) Request(Path, Method string, Body io.Reader, Headers map[string]string) (*http.Response, error) {
	if Method == "" {
		Method = "GET" // Set GET method
	}

	// Create request
	request, err := http.NewRequest(strings.ToUpper(Method), fmt.Sprintf("%s%s", api.Hostname, Path), Body)
	if err != nil {
		return nil, err
	}

	// Add headers to request
	if request.Header == nil {
		request.Header = http.Header{}
	}

	for key, value := range Headers {
		request.Header.Set(key, value) // Add Header to request
	}

	// Add token if not empty
	if api.Token != "" {
		request.Header.Set("Authorization", fmt.Sprintf("Token %s", api.Token))
	}

	return (&http.Client{}).Do((request)) // Make request
}