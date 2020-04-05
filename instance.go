package main

import (
	"time"
	"context"
	"io/ioutil"
	"net/http"
)

const (
	metadataBase   = "169.254.169.254"
	metadataURL    = "http://" + metadataBase + "/metadata/v1"
	metadataID     = metadataURL + "/id"
	metadataRegion = metadataURL + "/region"
	// metadataName   = metadataURL + "/hostname"
)

// Droplet contains information about the current host droplet
type Droplet struct {
	ID string
	Region string
}

// Instance gets the instance information from the droplet
func Instance(ctx Context) (*Droplet, error) {

	id, err := getURL(ctx, metadataID)
	if err != nil {
		return nil, err
	}
	region, err := getURL(ctx, metadataRegion)
	if err != nil {
		return nil, err
	}
	return &Droplet{ID: id, Region: region}, nil
}

// IsDroplet is a simple check to see if code is being executed on a DigitalOcean droplet or not
func IsDroplet(ctx Context) (bool, error) {
	_, err := getURL(ctx, metadataURL)
	if err != nil {
		return false, nil
	}
	return true, nil
}

func getURL(ctx Context, url string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := doRequest(ctx, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	id, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(id), nil
}

func doRequest(ctx Context, req *http.Request) (*http.Response, error) {
	return doRequestWithClient(ctx, http.DefaultClient, req)
}

func doRequestWithClient(
	ctx Context,
	client *http.Client,
	req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, 5000 * time.Millisecond)
	defer cancel()

	req = req.WithContext(ctx)
	return client.Do(req)
}
