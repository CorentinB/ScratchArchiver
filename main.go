package main

import (
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-resty/resty/v2"
	"github.com/paulbellamy/ratecounter"
	"github.com/philippgille/gokv/leveldb"
	"github.com/remeh/sizedwaitgroup"
	"github.com/sirupsen/logrus"
)

type Item struct {
	Exist bool
	ID    string
	User  string
}

func testID(ID string) *Item {
	var newItem = new(Item)
	newItem.ID = ID

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

		// If we are being rate limited, retry
		if resp.StatusCode == 429 {
			return testID(ID)
		}

		newItem.Exist = false
		return newItem
	}

	// We parse the username and fill the structure
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"id":    ID,
			"error": err,
		}).Debug("Response parsing failed, retrying..")
		resp.Body.Close()
		return testID(ID)
	}
	resp.Body.Close()

	newItem.Exist = true
	newItem.User = doc.Find("#view > div > div.inner > div.flex-row.preview-row.force-row > div.flex-row.project-header > div > a").Text()

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
	seencheck.WriteChan = make(chan *Item)

	seencheck.SeenCount.Incr(linesInFile("./IDs.txt"))

	seencheck.SeenDB, err = leveldb.NewStore(leveldb.Options{Path: "./database/seen"})
	if err != nil {
		logrus.Fatal(err)
	}
	defer seencheck.SeenDB.Close()

	go func() {
		os.MkdirAll(arguments.OutputDir, os.ModePerm)

		for item := range seencheck.WriteChan {
			f, err := os.OpenFile(item.User+".txt",
				os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				logrus.Fatal(err)
			}

			if _, err := f.WriteString(item.ID + "\n"); err != nil {
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

			if item.Exist == true {
				seencheck.Seen(item)

				logrus.WithFields(logrus.Fields{
					"id":         ID,
					"id/s":       seencheck.SeenRate.Rate(),
					"totalFound": seencheck.SeenCount.Value(),
				}).Info("New ID found")
			}
		}(strconv.Itoa(ID), &wg)
	}
}
