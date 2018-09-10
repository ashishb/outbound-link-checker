## Introduction

This simple tool allows you to list all outbound links from your domain to catch suspicious and spammy links. You can whitelist domains interactively to mark them as safe and ignore all the outbound links to them. 

## Some history behind this tool

My personal website was compromised, most likely due to password-reuse and the attacker carefully inserted a few spammy links for SEO to certain websites. I caught them on one-off basis and then realized that the best way to deal with these is to be able to list all the outbound links from my website and whitelist the domains one-by-one. Unfortunately, I did not find a single tool which can do this for me, so, I decided to make one. This tool preserves the domain whitelist file, so that, if I re-run the code a few months from now, then I have to only check the newly added links. I caught many more spammy links using this approach. Thinking that other website/blog owners are facing the same issue, I decided to put this tool out in public.

## Sample Usage

`go run outbound-link-checker.go --domain google.com --starting-url https://google.com -num-url-crawl-limit 10`

## Parameters

```
-domain string
    	The domain of the website, everything not on this domain will be considered outbound, don't prefix www in the front, for example, ashishb.net
-domains-whitelist-file string
  	  	A file containing a new-line separated white-listed domains, links to these domains will be ignored, any empty lines or lines starting with "//" in this file will be ignored as well
-interactive
    	Allows you to interactively add new domains to the list as they are encountered (default true)
-num-concurrent-crawls int
    	Number of concurrent requests to the website (Default: 20) (default 20)
-num-retry int
    	Number of retry attempts to fetch a URL (Default: 3) (default 3)
-num-url-crawl-limit int
    	Number of urls to crawl (default: unlimited) (default -1)
-starting-url string
    	The starting url to start the crawl from. Usually, the URL of the homepage, for example, https://ashishb.net
```