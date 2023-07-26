package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type pageInformation struct {
	pageNum int
	title   string
	user    string
	view    int
	link    string
}

var baseURL string = "https://www.inven.co.kr/board/ff14/4337?p="

func checkErr(err error) {
	if err != nil {
		fmt.Println(err.Error())
		log.Fatalln(err)
	}
}

func checkCode(res *http.Response) {
	if res.StatusCode != 200 {
		log.Fatalln("Request failed with Status:", res.StatusCode)
	}
}

func checkPageAvailable(url string, retry int) bool {
	res, err := http.Get(url)

	if err != nil {
		if retry > 0 {
			return checkPageAvailable(url, retry-1)
		} else {
			return false
		}
	}

	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		if retry > 0 {
			return checkPageAvailable(url, retry-1)
		} else {
			return false
		}
	}

	if doc.Find("div.board-list table tbody tr td div.no-result").Length() != 0 {
		return false
	}

	return true
}

func getPages() int {
	res, err := http.Get(baseURL)

	checkErr(err)
	checkCode(res)

	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	checkErr(err)

	numList := doc.Find("tbody tr.lgtm td.num span")
	if numList.Length() == 0 {
		log.Fatalln("No pages found")
	}

	maxNum := numList.First().Text()

	// convert string to int
	maxNumInt, err := strconv.Atoi(maxNum)
	maxNumInt = maxNumInt / 30
	checkErr(err)

	for i := maxNumInt; i > 0; i-- {
		if checkPageAvailable(baseURL+fmt.Sprintf("%v", i), 20) {
			return i
		} else {
			continue
		}
	}

	return 0
}

func getPageTitle(url string, retry int) ([]pageInformation, error) {
	fmt.Println("Requesting from : ", url)
	res, err := http.Get(url)

	if err != nil {
		if retry > 0 {
			return getPageTitle(url, retry-1)
		}

		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		res.Body.Close()
		if retry > 0 {
			return getPageTitle(url, retry-1)
		}
		return nil, err
	}

	numList := doc.Find("div.board-list table tbody tr").Clone()

	res.Body.Close()

	pages := []pageInformation{}

	numList.Each(func(i int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Find("td.tit div div a").Clone().Children().Remove().End().Text())
		link, exists := s.Find("td.tit div div a").Attr("href")
		if !exists {
			// handle error
		}
		pageNum, err := strconv.Atoi(s.Find("td.num span").Text())
		if err != nil {
			// handle error
		}
		user := s.Find("td.user span").Text()
		// view, err := strconv.Atoi(s.Find("td.view").Text())
		view, err := strconv.Atoi(strings.Replace(s.Find("td.view").Text(), ",", "", -1))
		if err != nil {
			// handle error
		}

		pageInfo := &pageInformation{
			pageNum: pageNum,
			title:   title,
			user:    user,
			view:    view,
			link:    link,
		}

		pages = append(pages, *pageInfo)
	})

	return pages, nil
}

func goroutineMethod(pageNum int, c chan []pageInformation) {
	pages, err := getPageTitle(baseURL+fmt.Sprintf("%v", pageNum), 20)
	if err != nil {
		log.Println(err)
		c <- nil
	} else {
		c <- pages
	}
}

func writePages(pages *[]pageInformation) {
	file, err := os.Create("pages.csv")
	checkErr(err)

	w := csv.NewWriter(file)
	defer w.Flush()
	headers := []string{"No.", "Title", "User", "View", "Link"}

	wErr := w.Write(headers)
	checkErr(wErr)

	for _, page := range *pages {
		pageInfo := []string{fmt.Sprintf("%v", page.pageNum), page.title, page.user, fmt.Sprintf("%v", page.view), page.link}
		wErr := w.Write(pageInfo)
		checkErr(wErr)
	}
}

// FIX: 왜 goroutine을 사용하면 에러 발생하는가?
var goroutineOption = true

func main() {
	results := []pageInformation{}
	maxPageNum := getPages()
	fmt.Println(maxPageNum)

	time.Sleep(time.Second * 5)

	if goroutineOption {
		c := make(chan []pageInformation)

		for i := 1; i <= maxPageNum; i++ {
			go goroutineMethod(i, c)
		}

		for i := 1; i <= maxPageNum; i++ {
			pages := <-c
			results = append(results, pages...)
		}
	} else {
		for i := 1; i <= maxPageNum; i++ {
			pages, err := getPageTitle(baseURL+fmt.Sprintf("%v", i), 20)
			if err != nil {
				log.Println(err)
			} else {
				results = append(results, pages...)
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].pageNum < results[j].pageNum
	})

	writePages(&results)
}
