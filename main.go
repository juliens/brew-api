package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"
)

type Cask struct {
	Token     string        `json:"token"`
	FullToken string        `json:"full_token"`
	OldTokens []interface{} `json:"old_tokens"`
	Tap       string        `json:"tap"`
	Name      []string      `json:"name"`
	Desc      string        `json:"desc"`
	Homepage  string        `json:"homepage"`
	Url       string        `json:"url"`
	UrlSpecs  struct {
		Verified string `json:"verified"`
	} `json:"url_specs"`
	Version            string        `json:"version"`
	Installed          interface{}   `json:"installed"`
	InstalledTime      interface{}   `json:"installed_time"`
	BundleVersion      interface{}   `json:"bundle_version"`
	BundleShortVersion interface{}   `json:"bundle_short_version"`
	Outdated           bool          `json:"outdated"`
	Sha256             string        `json:"sha256"`
	Artifacts          interface{}   `json:"artifacts"`
	Caveats            interface{}   `json:"caveats"`
	DependsOn          interface{}   `json:"depends_on"`
	ConflictsWith      interface{}   `json:"conflicts_with"`
	Container          interface{}   `json:"container"`
	AutoUpdates        interface{}   `json:"auto_updates"`
	Deprecated         bool          `json:"deprecated"`
	DeprecationDate    interface{}   `json:"deprecation_date"`
	DeprecationReason  interface{}   `json:"deprecation_reason"`
	Disabled           bool          `json:"disabled"`
	DisableDate        interface{}   `json:"disable_date"`
	DisableReason      interface{}   `json:"disable_reason"`
	TapGitHead         string        `json:"tap_git_head"`
	Languages          []interface{} `json:"languages"`
	RubySourcePath     string        `json:"ruby_source_path"`
	RubySourceChecksum interface{}   `json:"ruby_source_checksum"`
	Variations         interface{}   `json:"variations"`
}

var cacheCasks map[Key]*Cask

type Key struct {
	Token   string
	Version string
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("not enough argument")
	}

	caskJsonUrl := os.Args[1]
	resp, err := http.Get(caskJsonUrl)
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode != 200 {
		log.Fatal(resp.Status)
	}

	casks := []*Cask{}
	err = json.NewDecoder(resp.Body).Decode(&casks)
	if err != nil {
		log.Fatal(err)
	}

	readCacheCasks := []*Cask{}
	file, err := os.ReadFile("./cask.json")
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(file, &readCacheCasks)
	if err != nil {
		log.Fatal(err)
	}

	cacheCasks = make(map[Key]*Cask)
	for _, cask := range readCacheCasks {
		cacheCasks[Key{cask.Token, cask.Version}] = cask
	}

	result := make(chan *Cask)

	finished := make(chan struct{})
	go func() {
		newCasks := []*Cask{}
		for response := range result {
			newCasks = append(newCasks, response)
			// fmt.Printf("%d / %d  handled\n", len(newCasks), len(casks))
		}

		slices.SortFunc(newCasks, func(a, b *Cask) int {
			return strings.Compare(a.Token, b.Token)
		})

		cacheFile, err := os.OpenFile("./cask.json", os.O_WRONLY, 0x777)
		if err != nil {
			log.Fatal(err)
		}

		encoder := json.NewEncoder(cacheFile)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(newCasks)
		if err != nil {
			log.Fatal(err)
		}

		close(finished)
	}()

	workers := make(chan *Cask)
	wg := sync.WaitGroup{}
	for range 100 {
		go func() {
			wg.Add(1)
			defer wg.Done()
			for worker := range workers {
				result <- HandleHashRequest(worker)
			}
		}()
	}

	for _, cask := range casks {
		if cask.Token == "discord" {
			fmt.Println(cask)
		}
		workers <- cask
	}
	close(workers)
	wg.Wait()
	close(result)
	<-finished
}

func getHash(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, "nix-prefetch-url", fmt.Sprintf("%s", url)).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func HandleHashRequest(cask *Cask) *Cask {
	if cask.Token == "discord" {
		fmt.Printf("%+v\n", cask)
	}
	if cask.Sha256 != "no_check" {
		return cask
	}

	if cachedCask, ok := cacheCasks[Key{cask.Token, cask.Version}]; ok {
		if cachedCask.Sha256 == "error" || cask.Version != "latest" {
			return cachedCask
		}
	}

	hash, err := getHash(cask.Url)
	if err != nil {
		cask.Sha256 = "error"
		return cask
	}

	fmt.Printf("%s %s =>   updated\n", cask.Token, cask.Version)
	cask.Sha256 = hash
	return cask
}
