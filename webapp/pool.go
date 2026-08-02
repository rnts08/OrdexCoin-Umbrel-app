package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
)

const (
	poolAPIURL      = "https://api.miningcrypto.online/OrdexCoin/"
	poolPageURL     = "https://pool.ordexcoin.com/"
	nestExTickerURL = "https://trade.nestex.one/api/cg/tickers/OXC_USDT"
)

// blockRewardRe matches the block reward on the pool page, e.g.
// <p id="blockReward">20.00 OXC</p>
var blockRewardRe = regexp.MustCompile(`id="blockReward">\s*([0-9.]+)\s*OXC`)

// poolStats aggregates live public data from the OrdexCoin pool and
// livecoinwatch. Every field is optional: when an upstream is unreachable the
// field is simply omitted so the UI never sees a hard error.
type poolStats struct {
	NetworkHashrate *float64 `json:"networkHashrate,omitempty"`
	BlockReward     *string  `json:"blockReward,omitempty"`
	PriceUSD        *float64 `json:"priceUsd,omitempty"`
}

// handlePool proxies three external, non-critical data sources: the pool's
// network hashrate, the current block reward (grabbed from the pool page), and
// the OXC price from the NestEx exchange. All fetches run concurrently with a
// short timeout; failures never produce an HTTP error, they just leave the
// field out.
func (s *Server) handlePool(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 8 * time.Second}

	stats := &poolStats{}
	var wg sync.WaitGroup
	var mu sync.Mutex

	wg.Add(3)

	go func() {
		defer wg.Done()
		if v, ok := fetchPoolHashrate(ctx, client); ok {
			mu.Lock()
			stats.NetworkHashrate = v
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		if v, ok := fetchPoolBlockReward(ctx, client); ok {
			mu.Lock()
			stats.BlockReward = v
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		if v, ok := fetchNestExPrice(ctx, client); ok {
			mu.Lock()
			stats.PriceUSD = v
			mu.Unlock()
		}
	}()

	wg.Wait()
	writeJSON(w, http.StatusOK, stats)
}

// fetchPoolHashrate reads the network hashrate from the pool API.
func fetchPoolHashrate(ctx context.Context, client *http.Client) (*float64, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, poolAPIURL, nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		slog.Debug("pool API unreachable", "error", err)
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}

	// The pool API serves its JSON brotli-compressed (Content-Encoding: br).
	var reader io.Reader = resp.Body
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "br") {
		reader = brotli.NewReader(resp.Body)
	}
	body, err := io.ReadAll(io.LimitReader(reader, 1<<20))
	if err != nil {
		return nil, false
	}

	var doc struct {
		Body struct {
			Primary struct {
				Network struct {
					Hashrate float64 `json:"hashrate"`
				} `json:"network"`
			} `json:"primary"`
		} `json:"body"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, false
	}
	if doc.Body.Primary.Network.Hashrate <= 0 {
		return nil, false
	}
	return &doc.Body.Primary.Network.Hashrate, true
}

// fetchPoolBlockReward grabs the block reward straight from the pool page.
func fetchPoolBlockReward(ctx context.Context, client *http.Client) (*string, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, poolPageURL, nil)
	if err != nil {
		return nil, false
	}

	resp, err := client.Do(req)
	if err != nil {
		slog.Debug("pool page unreachable", "error", err)
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, false
	}

	m := blockRewardRe.FindStringSubmatch(string(body))
	if len(m) < 2 {
		return nil, false
	}
	reward := strings.TrimSpace(m[1]) + " OXC"
	return &reward, true
}

// fetchNestExPrice queries the NestEx exchange for the OXC/USDT last price.
func fetchNestExPrice(ctx context.Context, client *http.Client) (*float64, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, nestExTickerURL, nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		slog.Debug("nestex unreachable", "error", err)
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, false
	}

	var doc struct {
		LastPrice string `json:"last_price"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, false
	}
	price, err := strconv.ParseFloat(strings.TrimSpace(doc.LastPrice), 64)
	if err != nil || price <= 0 {
		return nil, false
	}
	return &price, true
}
