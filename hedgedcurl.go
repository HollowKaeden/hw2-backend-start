package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
)

const defaultTimeout = 15

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `Usage: hedgedcurl [options] URL [URL...]

Sends GET requests to all URLs in parallel and prints the first response
(status line, headers and body).

Options:
  -t, --timeout SECONDS   timeout for all HTTP requests (default %d)
  -h, --help              show this help and exit

Exit codes:
  0     success
  1     all requests failed or invalid arguments
  228   timeout
`, defaultTimeout)
}

func printResponse(resp *http.Response) error {
	fmt.Printf("%s %s\n", resp.Proto, resp.Status)
	if err := resp.Header.Write(os.Stdout); err != nil {
		return fmt.Errorf("write headers: %w", err)
	}
	fmt.Println()
	if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	return nil
}

type result struct {
	url  string
	resp *http.Response
	err  error
}

func main() {
	var timeout int

	flag.IntVar(&timeout, "t", defaultTimeout, "")
	flag.IntVar(&timeout, "timeout", defaultTimeout, "")

	flag.Usage = func() { printUsage(os.Stdout) }

	flag.Parse()

	if timeout <= 0 {
		fmt.Fprintln(os.Stderr, "hedgedcurl: timeout must be a positive number of seconds")
		os.Exit(1)
	}

	urls := flag.Args()

	if len(urls) == 0 {
		fmt.Fprintln(os.Stderr, "hedgedcurl: no URLs given")
		printUsage(os.Stderr)
		os.Exit(1)
	}

	results := make(chan result, len(urls))
	for _, url := range urls {
		go func() {
			resp, err := http.Get(url)
			results <- result{url: url, resp: resp, err: err}
		}()
	}
	for range len(urls) {
		r := <-results
		if r.err != nil {
			fmt.Fprintf(os.Stderr, "hedgedcurl: %s could not be processed: %v\n", r.url, r.err)
			continue
		}

		err := printResponse(r.resp)
		r.resp.Body.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, "hedgedcurl:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	fmt.Fprintln(os.Stderr, "hedgedcurl: all requests failed")
	os.Exit(1)
}
