package ldclient

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gregjones/httpcache"
)

// SDK endpoints
const (
	LatestFlagsPath    = "/sdk/latest-flags"
	LatestSegmentsPath = "/sdk/latest-segments"
	LatestAllPath      = "/sdk/latest-all"
)

type requestor struct {
	sdkKey     string
	httpClient *http.Client
	config     Config
}

func newRequestor(sdkKey string, config Config, httpClient *http.Client) *requestor {
	var decoratedClient http.Client
	if httpClient != nil {
		decoratedClient = *httpClient
	} else {
		decoratedClient = *config.newHTTPClient()
	}
	decoratedClient.Transport = &httpcache.Transport{
		Cache:               httpcache.NewMemoryCache(),
		MarkCachedResponses: true,
		Transport:           decoratedClient.Transport,
	}

	httpRequestor := requestor{
		sdkKey:     sdkKey,
		httpClient: &decoratedClient,
		config:     config,
	}

	return &httpRequestor
}

func (r *requestor) requestAll() (allData, bool, error) {
	var data allData
	body, cached, err := r.makeRequest(LatestAllPath)
	if err != nil {
		return allData{}, false, err
	}
	if cached {
		return allData{}, true, nil
	}
	jsonErr := json.Unmarshal(body, &data)

	if jsonErr != nil {
		return allData{}, false, jsonErr
	}
	return data, cached, nil
}

func (r *requestor) requestResource(kind VersionedDataKind, key string) (VersionedData, error) {
	var resource string
	switch kind {
	case SegmentVersionedDataKind{}:
		resource = LatestSegmentsPath + "/" + key
	case FeatureFlagVersionedDataKind{}:
		resource = LatestFlagsPath + "/" + key
	default:
		return nil, fmt.Errorf("unexpected item type: %s", kind)
	}
	body, _, err := r.makeRequest(resource)
	if err != nil {
		return nil, err
	}
	item := kind.GetDefaultItem().(VersionedData)
	err = json.Unmarshal(body, item)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *requestor) makeRequest(resource string) ([]byte, bool, error) {
	req, reqErr := http.NewRequest("GET", r.config.BaseUri+resource, nil)
	if reqErr != nil {
		return nil, false, reqErr
	}
	url := req.URL.String()

	req.Header.Add("Authorization", r.sdkKey)
	req.Header.Add("User-Agent", r.config.UserAgent)

	res, resErr := r.httpClient.Do(req)

	if resErr != nil {
		return nil, false, resErr
	}

	defer func() {
		_, _ = ioutil.ReadAll(res.Body)
		_ = res.Body.Close()
	}()

	if err := checkForHttpError(res.StatusCode, url); err != nil {
		return nil, false, err
	}

	cached := res.Header.Get(httpcache.XFromCache) != ""

	body, ioErr := ioutil.ReadAll(res.Body)

	if ioErr != nil {
		return nil, false, ioErr
	}
	return body, cached, nil
}
