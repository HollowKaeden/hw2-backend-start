package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
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
  1     all requests failed, no URLs given or invalid timeout
  2     invalid command-line flags
  228   timeout
`, defaultTimeout)
}

func printResponse(resp *http.Response, body []byte) error {
	fmt.Printf("%s %s\n", resp.Proto, resp.Status)
	if err := resp.Header.Write(os.Stdout); err != nil {
		return fmt.Errorf("write headers: %w", err)
	}
	fmt.Println()
	if _, err := os.Stdout.Write(body); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	return nil
}

type result struct {
	resp *http.Response
	body []byte
	err  error
}

func main() {
	var timeout int
	var help bool

	flag.BoolVar(&help, "h", false, "")
	flag.BoolVar(&help, "help", false, "")
	flag.IntVar(&timeout, "t", defaultTimeout, "")
	flag.IntVar(&timeout, "timeout", defaultTimeout, "")

	flag.Usage = func() { printUsage(os.Stderr) }

	flag.Parse()

	if help {
		printUsage(os.Stdout)
		os.Exit(0)
	}
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	for _, url := range urls {
		if !strings.Contains(url, "://") {
			url = "http://" + url
		}
		go func() {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				results <- result{err: err}
				return
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				results <- result{err: err}
				return
			}
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				results <- result{err: fmt.Errorf("read body from %s: %w", url, err)}
				return
			}
			results <- result{resp: resp, body: body}
		}()
	}
	for range len(urls) {
		r := <-results
		if r.err != nil {
			fmt.Fprintln(os.Stderr, "hedgedcurl:", r.err)
			continue
		}

		if err := printResponse(r.resp, r.body); err != nil {
			fmt.Fprintln(os.Stderr, "hedgedcurl:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		fmt.Fprintf(os.Stderr, "hedgedcurl: timeout after %ds\n", timeout)
		os.Exit(228)
	}
	fmt.Fprintln(os.Stderr, "hedgedcurl: all requests failed")
	os.Exit(1)
}
