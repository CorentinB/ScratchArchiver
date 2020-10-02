package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/paulbellamy/ratecounter"
	"github.com/philippgille/gokv/leveldb"
	"github.com/remeh/sizedwaitgroup"
	"github.com/sirupsen/logrus"
)

func testID(ID string) *Project {
	var newItem = new(Project)

	proxyClient := resty.New()

	if len(arguments.Proxy) != 0 {
		proxyClient.SetProxy("socks5://" + arguments.Proxy)
	}

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

		// If we are being rate limited, retry
		if resp.StatusCode == 429 {
			return testID(ID)
		}

		return newItem
	}

	err = json.NewDecoder(resp.Body).Decode(&newItem)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"id":    ID,
			"error": err,
		}).Warning("Response decoding failed, retrying..")
		resp.Body.Close()
		return testID(ID)
	}
	resp.Body.Close()

	return newItem
}

func main() {
	argumentParsing(os.Args)

	var seencheck = new(Seencheck)
	var wg = sizedwaitgroup.New(arguments.Concurrency)
	var err error

	seencheck.Mutex = new(sync.Mutex)
	seencheck.SeenRate = ratecounter.NewRateCounter(1 * time.Second)
	seencheck.SeenCount = new(ratecounter.Counter)
	seencheck.WriteChan = make(chan *Project)

	seencheck.SeenCount.Incr(linesInFile("./IDs.txt"))

	seencheck.SeenDB, err = leveldb.NewStore(leveldb.Options{Path: "./database/seen"})
	if err != nil {
		logrus.Fatal(err)
	}
	defer seencheck.SeenDB.Close()

	go func() {
		os.MkdirAll(arguments.OutputDir, os.ModePerm)

		for item := range seencheck.WriteChan {
			os.MkdirAll(path.Join(arguments.OutputDir, item.Author.Username[:1]), os.ModePerm)
			f, err := os.OpenFile(path.Join(arguments.OutputDir, item.Author.Username[:1], item.Author.Username+".txt"),
				os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				logrus.Fatal(err)
			}

			if _, err := f.WriteString(strconv.Itoa(item.ID) + "\n"); err != nil {
				logrus.Warning(err)
			}

			f.Close()
		}
	}()

	logrus.Info("Starting IDs discovery..")
	for ID := 0; ID <= 5000000000; ID++ {
		if seencheck.IsSeen(strconv.Itoa(ID)) {
			continue
		}

		wg.Add()
		go func(ID string, wg *sizedwaitgroup.SizedWaitGroup) {
			defer wg.Done()

			item := testID(ID)

			seencheck.SeenDB.Set(strconv.Itoa(item.ID), true)

			// If the author ID didn't change, it means that the project
			// doesn't exist.
			if item.Author.ID == 0 {
				return
			}

			seencheck.Seen(item)
			logrus.WithFields(logrus.Fields{
				"id":         ID,
				"username":   item.Author.Username,
				"userID":     item.Author.ID,
				"id/s":       seencheck.SeenRate.Rate(),
				"totalFound": seencheck.SeenCount.Value(),
			}).Info("New ID found")
		}(strconv.Itoa(ID), &wg)
	}
}
