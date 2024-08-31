package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/ashishb/outbound-link-checker/internal/logger"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

const defaultCrawlPageLimit = -1 // Unlimited
const defaultMaxConcurrentCrawls = 20
const defaultMaxBodyFetchRetryCount = 3
const defaultDomainWhitelistFile = "./%s_whitelisted_outbound_domains.txt"

// These urls cannot be fetched by the crawler since they are either dead or block the crawler.
const defaultWhitelistedOutboundUrls = "./%s_whitelisted_outbound_urls_known_dead_or_blocked.txt"

var crawlPageLimit = flag.Int("num-url-crawl-limit", defaultCrawlPageLimit,
	"Number of urls to crawl (default: unlimited)")
var maxConcurrentCrawls = flag.Int("num-concurrent-crawls", defaultMaxConcurrentCrawls,
	fmt.Sprintf("Number of concurrent requests to the website (Default: %d)", defaultMaxConcurrentCrawls))
var maxBodyFetchRetryCount = flag.Int("num-retry", defaultMaxBodyFetchRetryCount,
	fmt.Sprintf("Number of retry attempts to fetch a URL (Default: %d)", defaultMaxBodyFetchRetryCount))
var interactive = flag.Bool("interactive", true, "Allows you to interactively add new domains to the list as they"+
	" are encountered")
var showDeadLinks = flag.Bool("show-dead-links", false, "Print outbound links which are dead now")
var domainWhitelistFile = flag.String("domains-whitelist-file",
	"",
	"A file containing a newline separated white-listed domains,"+
		" links to these domains will be ignored, any empty lines or lines starting with \"//\" in"+
		" this file will be ignored as well")
var knownDeadOrBlockedExternalUrlsFileName = flag.String("dead-external-urls",
	"",
	"A file containing a newline separated external urls which are not crawable say due to crawler blocking. "+
		"Any empty lines or lines starting with \"//\" in this file will be ignored as well")
var startingUrl = flag.String("starting-url", "",
	"The starting url to start the crawl from. Usually, the URL of the homepage"+
		", for example, https://ashishb.net")
var domain = flag.String("domain", "",
	"The domain of the website, everything not on this domain will be considered outbound,"+
		" don't prefix www in the front, for example, ashishb.net")

var _httpClient = &http.Client{
	Timeout: 60 * time.Second,
}

func main() {
	handleFlags()
	logger.ConfigureLogging(true)

	whitelistedDomains := initWhitelistedDomains()
	knownDeadOrBlockedExternalUrls := initKnownDeadOrBlockedExternalUrls()
	// Maps url1 -> url2 if url1 has a link to url2.
	outboundLinkMap := make(map[url.URL][]url.URL, 0)
	visitedMap := make(map[url.URL]bool)
	startUrlParsed, err := url.Parse(*startingUrl)
	if err != nil {
		panic("Invalid starting url")
	}

	crawl(*startUrlParsed, *domain, outboundLinkMap, visitedMap, *crawlPageLimit, knownDeadOrBlockedExternalUrls)
	printResults(outboundLinkMap, *domain, whitelistedDomains)
}

func handleFlags() {
	flag.Parse()
	if len(*domain) == 0 {
		panic("Missing domain parameter")
	}
	if len(*startingUrl) == 0 {
		panic("Missing starting url")
	}
	if len(*domainWhitelistFile) == 0 {
		*domainWhitelistFile = fmt.Sprintf(defaultDomainWhitelistFile, *domain)
	}
	if len(*knownDeadOrBlockedExternalUrlsFileName) == 0 {
		*knownDeadOrBlockedExternalUrlsFileName = fmt.Sprintf(defaultWhitelistedOutboundUrls, *domain)
	}
}

func initWhitelistedDomains() map[string]bool {
	dat, err := os.ReadFile(*domainWhitelistFile)
	if err != nil {
		log.Warn().
			Str("file", *domainWhitelistFile).
			Msg("Domain whitelist file does not exist, it will be created later")
	}
	whitelisted := make([]string, 0)
	whitelistCount := 0
	for _, line := range strings.Split(string(dat), "\n") {
		line = strings.Trim(line, " ")
		// Ignore empty lines
		if len(line) == 0 {
			continue
		}
		// Ignore comments
		if strings.HasPrefix(line, "//") {
			continue
		}
		whitelistCount++
		whitelisted = append(whitelisted, line)
	}
	log.Info().
		Str("file", *domainWhitelistFile).
		Int("count", whitelistCount).
		Msg("Domain whitelist file loaded")

	whitelistedDomains := make(map[string]bool, 0)
	for _, domain := range whitelisted {
		addDomainToWhiteList(whitelistedDomains, domain)
	}
	return whitelistedDomains
}

func initKnownDeadOrBlockedExternalUrls() map[url.URL]bool {
	dat, err := os.ReadFile(*knownDeadOrBlockedExternalUrlsFileName)
	if err != nil {
		panic(fmt.Sprintf("File does not exist: %s, create an empty file.\n", *knownDeadOrBlockedExternalUrlsFileName))
	}
	urls := make(map[url.URL]bool, 0)
	urlCount := 0
	for _, line := range strings.Split(string(dat), "\n") {
		line = strings.Trim(line, " ")
		// Ignore empty lines
		if len(line) == 0 {
			continue
		}
		// Ignore comments
		if strings.HasPrefix(line, "//") {
			continue
		}
		urlCount++
		url1, err := url.Parse(line)
		if err != nil {
			log.Error().
				Str("url", line).
				Err(err).
				Msg("Error parsing url")
			continue
		}
		urls[*url1] = true
	}
	return urls
}

func addDomainToWhiteList(whitelistedDomains map[string]bool, domain string) {
	whitelistedDomains[domain] = true
	whitelistedDomains["www."+domain] = true
}

// Global variables
var count = 0
var lock = sync.Mutex{}
var crawlCountLock = sync.Mutex{}
var runningCrawlCount = 0

func crawl(
	url1 url.URL,
	domain string,
	outboundLinkMap map[url.URL][]url.URL,
	visitedMap map[url.URL]bool,
	crawlPageLimit int,
	knownDeadOrWhitelistedExternalUrls map[url.URL]bool) {

	if !recordNewVisit(url1, visitedMap) {
		log.Debug().
			Str("url", url1.String()).
			Msg("Skipping already visited url")
		return
	}

	lock.Lock()
	count++
	countValue := count
	// Code breaker for testing
	if crawlPageLimit > 0 && countValue > crawlPageLimit {
		lock.Unlock()
		return
	}
	lock.Unlock()

	log.Info().
		Int("count", countValue).
		Int("limit", crawlPageLimit).
		Str("url", url1.String()).
		Msg("Crawling")

	// Fetch the body
	body, err := getBody(url1)
	if err != nil {
		log.Fatal().
			Str("url", url1.String()).
			Err(err).
			Msg("Internal dead link found")
		return
	}

	// Extract the urls
	urls := getUrls(string(body))
	log.Debug().
		Str("url", url1.String()).
		Int("count", len(urls)).
		Msg("Found urls")

	for _, url2 := range urls {
		if url2.Path == "" && url2.Host == "" {
			log.Debug().
				Str("url", url2.String()).
				Str("source", url1.String()).
				Msg("Host and Path is empty, skipping")
			continue
		}
		url2.Fragment = ""
		log.Info().
			Str("url", url2.String()).
			Msg("Visiting url")
		if url2.String() == "" {
			log.Fatal().
				Str("url", url2.String()).
				Str("source", url1.String()).
				Msg("Empty url, skipping")
			continue
		}
		if url2.Host == "" {
			log.Debug().
				Str("url", url2.String()).
				Str("source", url1.String()).
				Msg("Host is empty, setting it to source host")
			url2.Host = url1.Host
		}
		if url2.Scheme == "" {
			log.Debug().
				Str("url", url2.String()).
				Str("source", url1.String()).
				Msg("Scheme is empty, setting it to source scheme")
			url2.Scheme = url1.Scheme
		}

		recordLink(url1, url2, outboundLinkMap)
		inDomainUrl := belongsToDomain(url2, domain)
		if inDomainUrl {
			go crawl(url2, domain, outboundLinkMap, visitedMap, crawlPageLimit, knownDeadOrWhitelistedExternalUrls)
		} else {
			if *showDeadLinks && !knownDeadOrWhitelistedExternalUrls[url2] &&
				recordNewVisit(url2, visitedMap) {
				go checkIfAlive(url2, url1)
			}
		}
	}

	time.Sleep(time.Second)
	for {
		crawlCountLock.Lock()
		value := runningCrawlCount
		crawlCountLock.Unlock()
		if value > 0 {
			log.Debug().
				Int("value", value).
				Msg("Waiting for all crawls to finish")
			time.Sleep(time.Second)
		} else {
			return
		}
	}
}

func waitForCrawlCountAvailability() {
	for {
		crawlCountLock.Lock()
		value := runningCrawlCount
		crawlCountLock.Unlock()
		if value < *maxConcurrentCrawls {
			return
		}
		time.Sleep(time.Second)
	}
}
func incrementRunningCrawlCount() {
	crawlCountLock.Lock()
	defer crawlCountLock.Unlock()
	runningCrawlCount++
}

func decrementRunningCrawlCount() {
	crawlCountLock.Lock()
	defer crawlCountLock.Unlock()
	runningCrawlCount--
}

func recordLink(url1 url.URL, url2 url.URL, outboundLinkMap map[url.URL][]url.URL) {
	lock.Lock()
	defer lock.Unlock()
	if outboundLinkMap[url1] == nil {
		outboundLinkMap[url1] = make([]url.URL, 0)
	}
	outboundLinkMap[url1] = append(outboundLinkMap[url1], url2)
}

func recordNewVisit(url url.URL, visitedMap map[url.URL]bool) bool {
	lock.Lock()
	defer lock.Unlock()
	if visitedMap[url] {
		return false
	} else {
		visitedMap[url] = true
		return true
	}
}

func belongsToDomain(url2 url.URL, domain string) bool {
	hostname := url2.Host
	return hostname == "" ||
		strings.Compare(hostname, domain) == 0 ||
		strings.Compare(hostname, "www."+domain) == 0
}

func getBody(url url.URL) ([]byte, error) {
	waitForCrawlCountAvailability()
	incrementRunningCrawlCount()
	defer decrementRunningCrawlCount()

	var err error
	retryCount := 0
	for retryCount < *maxBodyFetchRetryCount {
		retryCount++
		time.Sleep(time.Duration((retryCount - 1) * 1000 * 1000 * 1000))
		response, err1 := _httpClient.Get(url.String())
		if err1 != nil {
			log.Warn().
				Int("retryCount", retryCount).
				Str("url", url.String()).
				Err(err1).
				Msg("Failed to fetch")
			err = err1
			continue
		}
		defer response.Body.Close()
		bodyBytes, err2 := io.ReadAll(response.Body)
		if err2 != nil {
			log.Warn().
				Int("retryCount", retryCount).
				Str("url", url.String()).
				Err(err2).
				Msg("Failed to read body")
			err = err2
			continue
		}
		return bodyBytes, nil
	}
	return nil, err
}

func checkIfAlive(externalUrl url.URL, sourceUrl url.URL) {
	waitForCrawlCountAvailability()
	incrementRunningCrawlCount()
	defer decrementRunningCrawlCount()
	response, err := _httpClient.Get(externalUrl.String())
	if err != nil {
		log.Err(err).
			Str("url", externalUrl.String()).
			Str("source", sourceUrl.String()).
			Msg("Error while fetching")
		return
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		log.Err(err).
			Str("url", externalUrl.String()).
			Str("source", sourceUrl.String()).
			Int("statusCode", response.StatusCode).
			Msg("Error while fetching")
	}
}

// Hacky way to get links from HTML page
var linkRegEx = regexp.MustCompile(`<a.*?href=(.*?)[\s>]`)

func getUrls(htmlBody string) []url.URL {
	links := linkRegEx.FindAllStringSubmatch(htmlBody, -1)
	result := make([]url.URL, len(links))
	for i := range links {
		link := links[i][1]
		link = strings.Trim(link, "\"")
		link = strings.Trim(link, "'")

		// Internal links
		if strings.HasPrefix(link, "#") {
			continue
		}
		linkParsed, err := url.Parse(link)
		if err != nil {
			log.Error().
				Err(err).
				Str("link", link).
				Msg("Error parsing link")
			continue
		}
		result = append(result, *linkParsed)
	}
	return result
}

func printResults(outboundLinkMap map[url.URL][]url.URL, domain string, whitelistedDomains map[string]bool) {
	link := make(map[url.URL][]url.URL, 0)
	for url1, urls := range outboundLinkMap {
		for _, url2 := range urls {
			result := belongsToDomain(url2, domain)
			if result {
				continue
			}
			if whitelistedDomains[url2.Host] {
				continue
			}
			link[url2] = append(link[url2], url1)
		}
	}

	log.Info().
		Int("count", len(link)).
		Msg("Results")
	count := 0
	for url1, sourceUrls := range link {
		if len(sourceUrls) >= 1 {
			count++
			log.Info().
				Int("count", count).
				Int("total", len(link)).
				Str("url", url1.String()).
				Str("sourceUrls", sourceUrls[0].String()).
				Msg("URL")
			if *interactive {
				handleInteractively(url1, whitelistedDomains)
			}
		}
	}
}

// Whitelists domains interactively
func handleInteractively(parsedUrl url.URL, whitelistedDomains map[string]bool) {
	domain := parsedUrl.Host
	domain = strings.TrimPrefix(domain, "www.")
	if len(domain) == 0 {
		return
	}

	// Domain was whitelisted in this pass
	if whitelistedDomains[domain] {
		return
	}

	fmt.Printf("Whitelist domain \"%s\" [y/N]?", domain)
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	if strings.Compare(text, "y\n") == 0 {
		addDomainToWhiteList(whitelistedDomains, domain)
		file, err := os.OpenFile(*domainWhitelistFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			panic(
				fmt.Sprintf("Error opening file %s: %s\n", *domainWhitelistFile, err))
		}
		defer file.Close()
		w := bufio.NewWriter(file)
		fmt.Fprintln(w, domain)
		w.Flush()
		log.Info().
			Str("domain", domain).
			Msg("Domain whitelisted")
	} else {
		log.Info().
			Str("domain", domain).
			Msg("Domain not whitelisted")
	}
}
