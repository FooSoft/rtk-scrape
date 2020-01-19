package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/headzoo/surf"
	"github.com/headzoo/surf/agent"
	"github.com/headzoo/surf/browser"
)

type StoryEntry struct {
	Author        string `json:"author"`
	Content       string `json:"content"`
	ModifiedDate  string `json:"modifiedDate"`
	StarredCount  int    `json:"starredCount"`
	ReportedCount int    `json:"reportedCount"`
}

type KanjiEntry struct {
	Character   string         `json:"character"`
	Reading     string         `json:"reading"`
	FrameNumber int            `json:"frameNumber"`
	StrokeCount int            `json:"strokeCount"`
	Story       string         `json:"story"`
	Stories     StoryEntryList `json:"stories"`
}

type StoryEntryList []StoryEntry

func (e StoryEntryList) Len() int {
	return len(e)
}

func (e StoryEntryList) Less(i, j int) bool {
	return e[i].StarredCount > e[j].StarredCount
}

func (e StoryEntryList) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func login(br *browser.Browser, username, password string) error {
	loginUrl := "http://kanji.koohii.com/login"
	if err := br.Open(loginUrl); err != nil {
		return err
	}

	fm, err := br.Form("form")
	if err != nil {
		return errors.New("login form not found")
	}

	if err := fm.Input("username", username); err != nil {
		return err
	}

	if err := fm.Input("password", password); err != nil {
		return err
	}

	if err := fm.Submit(); err != nil {
		return err
	}

	if br.Title() == "Sign In - Kanji Koohii" {
		return errors.New("failed to sign in")
	}

	return nil
}

func scrape(br *browser.Browser, lookup string) (*KanjiEntry, error) {
	if err := br.Open(fmt.Sprintf("http://kanji.koohii.com/study/kanji/%s", lookup)); err != nil {
		return nil, err
	}

	var kanji KanjiEntry
	kanji.Character = strings.TrimSpace(br.Find("div.kanji span.cj-k").Text())
	kanji.Reading = strings.TrimSpace(br.Find("div.strokecount span.cj-k").Text())
	kanji.FrameNumber, _ = strconv.Atoi(strings.TrimSpace(br.Find("div.framenum").Text()))
	kanji.StrokeCount, _ = strconv.Atoi(strings.Split(strings.TrimSpace(br.Find("div.strokecount").Text()), " ")[0])

	if kanji.Story = strings.TrimSpace(br.Find("div#sv-textarea").Text()); kanji.Story == "[ click here to enter your story ]" {
		kanji.Story = ""
	}

	if matches := regexp.MustCompile(`\[(\d+)\]`).FindStringSubmatch(br.Find("div.strokecount").Text()); matches != nil {
		kanji.StrokeCount, _ = strconv.Atoi(matches[1])
	}

	br.Find("div.sharedstory").Each(func(i int, s *goquery.Selection) {
		var story StoryEntry
		story.Author = strings.TrimSpace(s.Find("div.sharedstory_author a").Text())
		story.Content = strings.TrimSpace(s.Find("div.story").Text())
		story.ModifiedDate = strings.TrimSpace(s.Find("div.lastmodified").Text())
		story.StarredCount, _ = strconv.Atoi(strings.TrimSpace(s.Find("a.JsStar").Text()))
		story.ReportedCount, _ = strconv.Atoi(strings.TrimSpace(s.Find("a.JsReport").Text()))
		kanji.Stories = append(kanji.Stories, story)
	})

	sort.Sort(kanji.Stories)

	return &kanji, nil
}

func load(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines, nil
}

func save(path string, kanjiList []*KanjiEntry) error {
	data, err := json.MarshalIndent(kanjiList, "", "   ")
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(path, data, 0644); err != nil {
		return err
	}

	return nil
}

func main() {
	var (
		username   = flag.String("username", "", "login username for kanji.koohii.com")
		password   = flag.String("password", "", "login password for kanji.koohii.com")
		firstFrame = flag.Int("firstFrame", 1, "kanji first frame")
		lastFrame  = flag.Int("lastFrame", 3030, "kanji last frame")
	)

	flag.Parse()

	args := flag.Args()
	if len(*username) == 0 || len(*password) == 0 || len(args) == 0 || *firstFrame >= *lastFrame {
		flag.Usage()
		os.Exit(2)
	}

	br := surf.NewBrowser()
	br.SetUserAgent(agent.Firefox())
	br.AddRequestHeader("Accept", "text/html")
	br.AddRequestHeader("Accept-Charset", "utf8")

	log.Println("logging in...")
	if err := login(br, *username, *password); err != nil {
		log.Fatal(err)
	}

	var lookups []string
	if len(args) >= 2 {
		log.Printf("loading from %s...", args[1])
		var err error
		if lookups, err = load(args[1]); err != nil {
			log.Fatal(err)
		}
	} else {
		for i := *firstFrame; i <= *lastFrame; i++ {
			lookups = append(lookups, strconv.Itoa(i))
		}
	}

	var kanjiList []*KanjiEntry
	for _, lookup := range lookups {
		log.Printf("scraping %s...", lookup)
		kanji, err := scrape(br, lookup)
		if err != nil {
			log.Fatal(err)
		}

		kanjiList = append(kanjiList, kanji)
		time.Sleep(2 * time.Second)
	}

	log.Printf("saving to %s...", args[0])
	if err := save(args[0], kanjiList); err != nil {
		log.Fatal(err)
	}
}
