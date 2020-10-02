package main

import (
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/paulbellamy/ratecounter"
	"github.com/philippgille/gokv/leveldb"
	"github.com/remeh/sizedwaitgroup"
	"github.com/sirupsen/logrus"
)

func testID(ID string) bool {
	proxyClient := resty.New()
	proxyClient.SetProxy("socks5://" + arguments.Proxy)

	req, err := http.NewRequest("GET", "https://api.scratch.mit.edu/projects/"+ID, nil)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"id":    ID,
			"error": err,
		}).Debug("Request creation failed, retrying..")
		return testID(ID)
	}

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
			"id":    ID,
			"error": err,
		}).Debug("Request execution failed, retrying..")
		return testID(ID)
	}

	if resp.StatusCode != 200 {
		resp.Body.Close()
		return false
	}

	resp.Body.Close()
	return true
}

func main() {
	argumentParsing(os.Args)

	var seencheck = new(Seencheck)
	var wg = sizedwaitgroup.New(arguments.Concurrency)
	var err error

	seencheck.Mutex = new(sync.Mutex)
	seencheck.SeenRate = ratecounter.NewRateCounter(1 * time.Second)
	seencheck.SeenCount = new(ratecounter.Counter)
	seencheck.WriteChan = make(chan string)

	seencheck.SeenCount.Incr(linesInFile("./IDs.txt"))

	seencheck.SeenDB, err = leveldb.NewStore(leveldb.Options{Path: "./database/seen"})
	if err != nil {
		logrus.Fatal(err)
	}
	defer seencheck.SeenDB.Close()

	go func() {
		f, err := os.OpenFile("IDs.txt",
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			logrus.Fatal(err)
		}
		defer f.Close()

		for line := range seencheck.WriteChan {
			if _, err := f.WriteString(line + "\n"); err != nil {
				logrus.Warning(err)
			}
		}
	}()

	logrus.Info("Starting IDs discovery..")
	for ID := 0; ID <= 5000000000; ID++ {
		wg.Add()
		go func(ID string, wg *sizedwaitgroup.SizedWaitGroup) {
			defer wg.Done()

			if testID(ID) == true {
				if seencheck.IsSeen(ID) == false {
					seencheck.Seen(ID)
				}

				logrus.WithFields(logrus.Fields{
					"id":         ID,
					"id/s":       seencheck.SeenRate.Rate(),
					"totalFound": seencheck.SeenCount.Value(),
				}).Info("New ID found")
			}
		}(strconv.Itoa(ID), &wg)
	}
}
