package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/schollz/progressbar/v3"
)

type Scraper struct {
	Timeout time.Duration
	Client  *http.Client
}

func NewScraper(timeout time.Duration) *Scraper {
	return &Scraper{
		Timeout: timeout,
		Client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (s *Scraper) GetHTML(path string) (string, error) {
	url := fmt.Sprintf("https://legends.dbz.space%s", path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (U; Linux x86_64) Gecko/20130401 Firefox/58.3")
	resp, err := s.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(bodyBytes), nil
	}
	return "", fmt.Errorf("HTTP status code: %d", resp.StatusCode)
}

func (s *Scraper) FetchLinks(html string) []string {
	var links []string
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		fmt.Println("Error loading HTML:", err)
		return links
	}
	doc.Find("div.chara.list a").Each(func(i int, s *goquery.Selection) {
		link, exists := s.Attr("href")
		if exists {
			links = append(links, link)
		}
	})
	return links
}

func (s *Scraper) GetCharacter(html string) Character {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		fmt.Println("Error loading HTML:", err)
		return Character{}
	}
	name := sanitizeString(doc.Find("div.head.name.large.img_back h1").Text())
	id := sanitizeString(doc.Find("div.head.name.id-right.small.img_back").Text())
	color := sanitizeString(strings.TrimSpace(doc.Find("div.element").Text()))
	rarity := sanitizeString(strings.TrimSpace(doc.Find("div.rarity").Text()))
	var tags []string
	doc.Find("span.ability.medium a").Each(func(i int, s *goquery.Selection) {
		tags = append(tags, sanitizeString(strings.TrimSpace(s.Text())))
	})
	mainAbilityName := sanitizeString(doc.Find("div.frm.form0 span.ability.medium").Text())
	mainAbilityEffect := sanitizeString(doc.Find("div.frm.form0 div.ability_text.small").Text())
	mainAbility := Ability{Name: mainAbilityName, Effect: mainAbilityEffect}
	var uniqueAbilities []Ability
	doc.Find("a#charaunique + div.ability_text div.frm.form0").Each(func(i int, s *goquery.Selection) {
		abilityName := sanitizeString(s.Find("span.ability.medium").Text())
		abilityEffect := sanitizeString(s.Find("div.ability_text.small").Text())
		uniqueAbilities = append(uniqueAbilities, Ability{Name: abilityName, Effect: abilityEffect})
	})
	var zenkaiAbilities []Ability
	doc.Find("a#charaunique + div.ability_text div.frm.form1").Each(func(i int, s *goquery.Selection) {
		abilityName := sanitizeString(s.Find("span.ability.medium").Text())
		abilityEffect := sanitizeString(s.Find("div.ability_text.small").Text())
		zenkaiAbilities = append(zenkaiAbilities, Ability{Name: abilityName, Effect: abilityEffect})
	})
	var ultraAbility *Ability
	ultraAbilityName := sanitizeString(doc.Find("a#charaultra + div.ability_text div.frm.form0 span.ability.medium").Text())
	ultraAbilityEffect := sanitizeString(doc.Find("a#charaultra + div.ability_text div.frm.form0 div.ability_text.small").Text())
	if ultraAbilityName != "" && ultraAbilityEffect != "" {
		ultraAbility = &Ability{Name: ultraAbilityName, Effect: ultraAbilityEffect}
	}
	baseStats := Stats{
		Power:      getIntFromAttribute(doc.Find("div.row.lvlbreak.lvb1 div.col div.val").AttrOr("raw", "")),
		Health:     getIntFromAttribute(doc.Find("div.row.lvlbreak.lvb1 div.col div.val").AttrOr("raw", "")),
		StrikeAtk:  getIntFromAttribute(doc.Find("div.row.lvlbreak.lvb1 div.col div.val").AttrOr("raw", "")),
		StrikeDef:  getIntFromAttribute(doc.Find("div.row.lvlbreak.lvb1 div.col div.val").AttrOr("raw", "")),
		BlastAtk:   getIntFromAttribute(doc.Find("div.row.lvlbreak.lvb1 div.col div.val").AttrOr("raw", "")),
		BlastDef:   getIntFromAttribute(doc.Find("div.row.lvlbreak.lvb1 div.col div.val").AttrOr("raw", "")),
	}
	maxStats := Stats{
		Power:      getIntFromAttribute(doc.Find("div.row.lvlbreak.lvb5000 div.col div.val").AttrOr("raw", "")),
		Health:     getIntFromAttribute(doc.Find("div.row.lvlbreak.lvb5000 div.col div.val").AttrOr("raw", "")),
		StrikeAtk:  getIntFromAttribute(doc.Find("div.row.lvlbreak.lvb5000 div.col div.val").AttrOr("raw", "")),
		StrikeDef:  getIntFromAttribute(doc.Find("div.row.lvlbreak.lvb5000 div.col div.val").AttrOr("raw", "")),
		BlastAtk:   getIntFromAttribute(doc.Find("div.row.lvlbreak.lvb5000 div.col div.val").AttrOr("raw", "")),
		BlastDef:   getIntFromAttribute(doc.Find("div.row.lvlbreak.lvb5000 div.col div.val").AttrOr("raw", "")),
	}
	imageURL, _ := doc.Find("img.cutin.trs0.form0").Attr("src")
	strikeInfo := sanitizeString(doc.Find("a#charastrike + div.ability_text.arts div.frm.form0 div.ability_text.small").Text())
	shotInfo := sanitizeString(doc.Find("a#charashot + div.ability_text.arts div.frm.form0 div.ability_text.small").Text())
	specialMoveName := sanitizeString(doc.Find("a#charaspecial_move + div.ability_text.arts div.frm.form0 span.ability.medium").Text())
	specialMoveEffect := sanitizeString(doc.Find("a#charaspecial_move + div.ability_text.arts div.frm.form0 div.ability_text.small").Text())
	specialMove := Ability{Name: specialMoveName, Effect: specialMoveEffect}
	specialSkillName := sanitizeString(doc.Find("a#charaspecial_skill + div.ability_text.arts div.frm.form0 span.ability.medium").Text())
	specialSkillEffect := sanitizeString(doc.Find("a#charaspecial_skill + div.ability_text.arts div.frm.form0 div.ability_text.small").Text())
	specialSkill := Ability{Name: specialSkillName, Effect: specialSkillEffect}
	var ultimateSkill *Ability
	ultimateSkillName := sanitizeString(doc.Find("a#charaultimate_skill + div.ability_text.arts div.frm.form0 span.ability.medium").Text())
	ultimateSkillEffect := sanitizeString(doc.Find("a#charaultimate_skill + div.ability_text.arts div.frm.form0 div.ability_text.small").Text())
	if ultimateSkillName != "" && ultimateSkillEffect != "" {
		ultimateSkill = &Ability{Name: ultimateSkillName, Effect: ultimateSkillEffect}
	}
	var zAbilities [4]ZAbility
	for i := 1; i <= 4; i++ {
		zAbilityTags := []string{}
		doc.Find(fmt.Sprintf("div.zability.z%d div.ability_text.medium", i)).Each(func(j int, s *goquery.Selection) {
			zAbilityTags = append(zAbilityTags, sanitizeString(strings.TrimSpace(s.Text())))
		})
		zAbilityEffect := sanitizeString(doc.Find(fmt.Sprintf("div.zability.z%d div.ability_text.medium", i)).Next().Text())
		zAbilities[i-1] = ZAbility{Tags: zAbilityTags, Effect: zAbilityEffect}
	}
	isLF := doc.Find("img.legends-limited").Length() > 0

	return Character{
		Name:           name,
		ID:             id,
		Color:          color,
		Rarity:         rarity,
		Tags:           tags,
		MainAbility:    mainAbility,
		UniqueAbility:  UniqueAbilities{StartAbilities: uniqueAbilities, ZenkaiAbilities: zenkaiAbilities},
		UltraAbility:   ultraAbility,
		BaseStats:      baseStats,
		MaxStats:       maxStats,
		StrikeInfo:     strikeInfo,
		ShotInfo:       shotInfo,
		ImageURL:       imageURL,
		SpecialMove:    specialMove,
		SpecialSkill:   specialSkill,
		UltimateSkill:  ultimateSkill,
		ZAbilities:     zAbilities,
		IsLF:           isLF,
	}
}

func getIntFromAttribute(attr string) int {
	if attr == "" {
		return 0
	}
	value, _ := strconv.Atoi(attr)
	return value
}

func sanitizeString(s string) string {
	// Remove leading and trailing whitespace and special characters
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\n\t\r")
	return s
}

func main() {
	scraper := NewScraper(5 * time.Second) // Increased timeout for slower scraping
	defer scraper.Client.CloseIdleConnections()

	url := "/characters/"
	html, err := scraper.GetHTML(url)
	if err != nil {
		fmt.Printf("Failed to fetch HTML: %v\n", err)
		return
	}

	// Extract links from HTML
	links := scraper.FetchLinks(html)

	var characters []Character
	var wg sync.WaitGroup

	bar := progressbar.Default(int64(len(links)), "Scraping")

	for _, link := range links {
		wg.Add(1)
		go func(link string) {
			defer wg.Done()
			html, err := scraper.GetHTML(link)
			if err != nil {
				fmt.Printf("Failed to fetch HTML for link %s: %v\n", link, err)
				return
			}
			character := scraper.GetCharacter(html)
			characters = append(characters, character)
			bar.Add(1)
			time.Sleep(1 * time.Second) // Introduce delay between requests
		}(link)
	}

	wg.Wait()

	// Convert characters to JSON and save to file
	jsonData, err := json.MarshalIndent(characters, "", "    ")
	if err != nil {
		fmt.Printf("Failed to marshal characters to JSON: %v\n", err)
		return
	}

	err = ioutil.WriteFile(".CHARACTER-STATS.json", jsonData, 0644)
	if err != nil {
		fmt.Printf("Failed to write JSON to file: %v\n", err)
		return
	}

	fmt.Println("Done!")
}

type Character struct {
	Name          string
	ID            string
	Color         string
	Rarity        string
	Tags          []string
	MainAbility   Ability
	UniqueAbility UniqueAbilities
	UltraAbility  *Ability
	BaseStats     Stats
	MaxStats      Stats
	StrikeInfo    string
	ShotInfo      string
	ImageURL      string
	SpecialMove   Ability
	SpecialSkill  Ability
	UltimateSkill *Ability
	ZAbilities    [4]ZAbility
	IsLF          bool
}

type Ability struct {
	Name   string
	Effect string
}

type UniqueAbilities struct {
	StartAbilities  []Ability
	ZenkaiAbilities []Ability
}

type Stats struct {
	Power     int
	Health    int
	StrikeAtk int
	StrikeDef int
	BlastAtk  int
	BlastDef  int
}

type ZAbility struct {
	Tags   []string
	Effect string
}
