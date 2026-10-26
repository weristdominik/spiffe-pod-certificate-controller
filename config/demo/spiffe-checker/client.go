package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

func (c *Checker) RunClient(ctx context.Context) error {
	transport := &http.Transport{
		TLSClientConfig:   c.clientTLS(),
		DisableKeepAlives: true,
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	ticker := time.NewTicker(c.Config.RequestInterval)
	defer ticker.Stop()

	for {
		if err := c.request(ctx, client); err != nil {
			log.Printf("request failed: %v", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (c *Checker) request(
	ctx context.Context,
	client *http.Client,
) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.Config.TargetURL,
		nil,
	)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	log.Printf("server response: %s", resp.Status)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server returned %s", resp.Status)
	}

	log.Println("authenticated request successful")
	return nil
}
