package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Banner struct {
	Title          string
	ImageURL       string
	StartDate      string
	EndDate        string
	FeaturedChars  []FeaturedCharacter
}

type FeaturedCharacter struct {
	Name  string
	Image string
}

type Scraper struct {
	BaseURL string
	Timeout time.Duration
	Client  *http.Client
}

func NewScraper(baseURL string, timeout time.Duration) *Scraper {
	return &Scraper{
		BaseURL: baseURL,
		Timeout: timeout,
		Client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (s *Scraper) GetBannerPaths() ([]string, error) {
	url := s.BaseURL
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (U; Linux x86_64) Gecko/20130401 Firefox/58.3")
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		var paths []string
		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			return nil, err
		}
		doc.Find("a[href^='/banner/']").Each(func(i int, s *goquery.Selection) {
			path, exists := s.Attr("href")
			if exists {
				paths = append(paths, path)
			}
		})
		return paths, nil
	}
	return nil, fmt.Errorf("HTTP status code: %d", resp.StatusCode)
}

func (s *Scraper) FetchBannerData(path string) Banner {
	url := s.BaseURL + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Banner{}
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (U; Linux x86_64) Gecko/20130401 Firefox/58.3")
	resp, err := s.Client.Do(req)
	if err != nil {
		return Banner{}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		var banner Banner
		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			return Banner{}
		}
		banner.Title = strings.TrimSpace(doc.Find("h2.text-center").First().Text())
		banner.ImageURL, _ = doc.Find("img.bannerimage").First().Attr("src")
		dateText := doc.Find("h5.text-center").First().Text()
		dates := strings.Split(dateText, "～")
		if len(dates) >= 2 {
			banner.StartDate = strings.TrimSpace(dates[0])
			banner.EndDate = strings.TrimSpace(dates[1])
		}
		doc.Find(".character-container .chara-listing").Each(func(i int, s *goquery.Selection) {
			charName := strings.TrimSpace(s.Find(".card-header.name").First().Text())
			charImage, _ := s.Find("img.carder").First().Attr("src")
			banner.FeaturedChars = append(banner.FeaturedChars, FeaturedCharacter{Name: charName, Image: charImage})
		})
		return banner
	}
	return Banner{}
}

func main() {
	baseURL := "https://dblegends.net"
	scraper := NewScraper(baseURL, 5*time.Second) // Increased timeout for slower scraping
	defer scraper.Client.CloseIdleConnections()

	paths, err := scraper.GetBannerPaths()
	if err != nil {
		fmt.Printf("Failed to fetch banner paths: %v\n", err)
		return
	}

	var banners []Banner
	var wg sync.WaitGroup

	for _, path := range paths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			banner := scraper.FetchBannerData(path)
			if banner.Title != "" {
				banners = append(banners, banner)
			}
		}(path)
	}

	wg.Wait()

	fmt.Println("Banners:")
	for _, banner := range banners {
		fmt.Println("Title:", banner.Title)
		fmt.Println("Image URL:", banner.ImageURL)
		fmt.Println("Start Date:", banner.StartDate)
		fmt.Println("End Date:", banner.EndDate)
		fmt.Println("Featured Characters:")
		for _, char := range banner.FeaturedChars {
			fmt.Printf("- Name: %s, Image: %s\n", char.Name, char.Image)
		}
		fmt.Println()
	}

	// Save banners to JSON file
	jsonData, err := json.MarshalIndent(banners, "", "    ")
	if err != nil {
		fmt.Printf("Failed to marshal banners to JSON: %v\n", err)
		return
	}

	file, err := os.Create(".BANNER_DATA.json")
	if err != nil {
		fmt.Printf("Failed to create JSON file: %v\n", err)
		return
	}
	defer file.Close()

	_, err = file.Write(jsonData)
	if err != nil {
		fmt.Printf("Failed to write JSON to file: %v\n", err)
		return
	}

	fmt.Println("Banner data saved to .BANNER_DATA.json")
}
