package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
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
	Version            string      `json:"version"`
	Installed          interface{} `json:"installed"`
	InstalledTime      interface{} `json:"installed_time"`
	BundleVersion      interface{} `json:"bundle_version"`
	BundleShortVersion interface{} `json:"bundle_short_version"`
	Outdated           bool        `json:"outdated"`
	Sha256             string      `json:"sha256"`
	Artifacts          []struct {
		App []string `json:"app,omitempty"`
		Zap []struct {
			Trash string `json:"trash"`
		} `json:"zap,omitempty"`
	} `json:"artifacts"`
	Caveats   interface{} `json:"caveats"`
	DependsOn struct {
		Macos struct {
			Field1 []string `json:">="`
		} `json:"macos"`
	} `json:"depends_on"`
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
	RubySourceChecksum struct {
		Sha256 string `json:"sha256"`
	} `json:"ruby_source_checksum"`
	Variations struct {
		Sonoma struct {
			Url    string `json:"url"`
			Sha256 string `json:"sha256"`
		} `json:"sonoma"`
		Ventura struct {
			Url    string `json:"url"`
			Sha256 string `json:"sha256"`
		} `json:"ventura"`
		Monterey struct {
			Url    string `json:"url"`
			Sha256 string `json:"sha256"`
		} `json:"monterey"`
		BigSur struct {
			Url    string `json:"url"`
			Sha256 string `json:"sha256"`
		} `json:"big_sur"`
		Catalina struct {
			Url    string `json:"url"`
			Sha256 string `json:"sha256"`
		} `json:"catalina"`
		Mojave struct {
			Url    string `json:"url"`
			Sha256 string `json:"sha256"`
		} `json:"mojave"`
		HighSierra struct {
			Url    string `json:"url"`
			Sha256 string `json:"sha256"`
		} `json:"high_sierra"`
		Sierra struct {
			Url    string `json:"url"`
			Sha256 string `json:"sha256"`
		} `json:"sierra"`
		ElCapitan struct {
			Url    string `json:"url"`
			Sha256 string `json:"sha256"`
		} `json:"el_capitan"`
	} `json:"variations"`
}

var cacheResponse map[string]HashResponse

func main() {

	cacheResponse = make(map[string]HashResponse)
	casks := []*Cask{}
	file, err := os.ReadFile("./cask.json")
	if err != nil {
		log.Fatal(err)
	}
	json.Unmarshal(file, &casks)

	cache, err := os.ReadFile("./cache.json")
	if err != nil {
		log.Fatal(err)
	}
	json.Unmarshal(cache, &cacheResponse)

	result := make(chan HashResponse)
	results := map[string]HashResponse{}
	go func() {
		for response := range result {
			results[response.Token] = response

			fmt.Printf("handle response %s %s Error: %s\n", response.Token, response.Hash, response.Err)

			new, err := json.Marshal(results)
			if err != nil {
				log.Fatal(err)
			}

			os.WriteFile("./cache.json", new, 0x777)
		}
	}()

	workers := make(chan Cask)
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
		if cask.Sha256 == "no_check" {
			workers <- *cask
		}
	}
	close(workers)
	wg.Wait()
	close(result)

	cache, err = os.ReadFile("./cache.json")
	if err != nil {
		log.Fatal(err)
	}
	json.Unmarshal(cache, &cacheResponse)

	for _, cask := range casks {
		if cask.Sha256 == "no_check" {
			if resp, ok := cacheResponse[cask.Token]; ok {
				if cask.Version == resp.Version {
					cask.Sha256 = resp.Hash
				}
			}

		}
	}

	new, err := json.Marshal(casks)
	if err != nil {
		log.Fatal(err)
	}

	os.WriteFile("./cask2.json", new, 0x777)

}

func getHash(url string) (string, error) {
	out, err := exec.Command("nix-prefetch-url", url).Output()
	if err != nil {
		fmt.Println(string(err.(*exec.ExitError).Stderr))
		return "", err
	}
	return string(out), nil
}

func HandleHashRequest(cask Cask) HashResponse {
	if cacheItem, ok := cacheResponse[cask.Token]; ok {
		if cacheItem.Hash != "" && cacheItem.Version == cask.Version {
			fmt.Println("FROM CACHE")
			return cacheItem
		}
	}
	hash, err := getHash(cask.Url)
	var msg string
	if err != nil {
		msg = err.Error()
	}
	if err != nil && errors.Is(err, &exec.ExitError{}) {
		msg = err.Error() + string(err.(*exec.ExitError).Stderr)
	}

	fmt.Println("MESSAGE " + msg)
	return HashResponse{
		Hash:    strings.TrimSpace(hash),
		Err:     msg,
		Token:   cask.Token,
		Version: cask.Version,
	}

}

type HashResponse struct {
	Token   string
	Version string
	Hash    string
	Err     string
}
