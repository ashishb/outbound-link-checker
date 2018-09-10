## Introduction

This simple tool allows you to list all outbound links from your domain to catch suspicious and spammy links. You can whitelist domains interactively to mark them as safe and ignore all the outbound links to them. 

## Some history behind this tool

My personal website was compromised, most likely due to password-reuse and the attacker carefully inserted a few spammy links for SEO to certain websites. I caught them on one-off basis and then realized that the best way to deal with these is to be able to list all the outbound links from my website and whitelist the domains one-by-one. Unfortunately, I did not find a single tool which can do this for me, so, I decided to make one. This tool preserves the domain whitelist file is preserved, so that, if I re-run the code a few months from now, then I have to only check the newly added links.

## Sample Usage

`go run outbound-link-checker.go 
	--domain ashishb.net
	--starting-url https://ashishb.net`
	
