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
	maxNumInt = maxNumInt/30 + 1 // page당 30개의 게시글이 있음
	checkErr(err)

	for i := maxNumInt; i > 0; i-- {
		// 페이지 별 게시글이 존재하는지 확인
		// 게시글이 삭제된 경우, num은 해당 번호를 건너뛰기 때문에 마지막 page는 존재하지 않을 수 있음
		// 게시글의 num은 1씩 증가하고, 중복되지 않으므로 마지막 page 뒤의 게시글은 존재할 수 없음
		// 따라서 마지막 Page부터 게시글이 존재하는지 확인하고, 최초로 게시글이 존재하는 page를 리턴합니다.
		if checkPageAvailable(baseURL+fmt.Sprintf("%v", i), 20) { // 해당 페이지에 게시글이 존재하는지 확인
			return i // 게시글이 존재한다면 page num을 리턴합니다.
		} else {
			continue // 아니라면 반복
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
			/* handle error */
		}

		pageNum, err := strconv.Atoi(s.Find("td.num span").Text())
		if err != nil {
			/* handle error */
		}

		user := s.Find("td.user span").Text()

		view, err := strconv.Atoi(strings.Replace(s.Find("td.view").Text(), ",", "", -1))
		if err != nil {
			/* handle error */
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

func goroutineMethod(pageNum int, c chan<- []pageInformation) {
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
// Response: goroutine 속 map의 원본을 포인터로 전달하여 수정하도록 쓰여진 코드이기 때문에 발생하는 문제같다. 채널을 통해 데이터를 전달받아서 메인 함수에서 취합하니 해결되었다.
var goroutineOption = true

func main() {
	results := []pageInformation{}
	maxPageNum := getPages() // 최대 page를 계산해서 받아오는 부분
	fmt.Println(fmt.Sprint(maxPageNum) + "pages found")

	c := make(chan []pageInformation)

	for i := 1; i <= maxPageNum; i++ {
		go goroutineMethod(i, c)
	}

	for i := 1; i <= maxPageNum; i++ {
		pages := <-c
		results = append(results, pages...)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].pageNum < results[j].pageNum
	})

	writePages(&results)
}
