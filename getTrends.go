package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"
)

func getTrends(limit, offset, mode string) []string {
	proxyClient := resty.New()

	if len(arguments.Proxy) != 0 {
		proxyClient.SetProxy("socks5://" + arguments.Proxy)
	}

	req, err := http.NewRequest("GET", "https://api.scratch.mit.edu/explore/projects", nil)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"offset": offset,
			"limit":  limit,
			"mode":   mode,
		}).Warning("Request creation failed, retrying..")
		return getTrends(limit, offset, mode)
	}

	q := req.URL.Query()
	q.Add("limit", limit)
	q.Add("offset", offset)
	q.Add("mode", mode)
	q.Add("q", "*")
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Authority", "api.scratch.mit.edu")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/85.0.4183.121 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Origin", "https://scratch.mit.edu")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Referer", "https://scratch.mit.edu/")
	req.Header.Set("Accept-Language", "fr-FR,fr;q=0.9,en-US;q=0.8,en;q=0.7")

	resp, err := proxyClient.GetClient().Do(req)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"offset": offset,
			"limit":  limit,
			"mode":   mode,
		}).Warning("Request execution failed, retrying..")
		return getTrends(limit, offset, mode)
	}

	var trending = new(Trending)
	err = json.NewDecoder(resp.Body).Decode(&trending)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"offset": offset,
			"limit":  limit,
			"mode":   mode,
			"error":  err,
		}).Warning("Response decoding failed, retrying..")
		resp.Body.Close()
		return getTrends(limit, offset, mode)
	}

	var IDs []string
	for _, result := range *trending {
		IDs = append(IDs, strconv.Itoa(result.ID))
	}

	resp.Body.Close()
	return IDs
}
