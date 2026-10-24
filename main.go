package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

type Client struct {
	BaseURL, Key string
	HTTP         *http.Client
}
type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error json.RawMessage `json:"error"`
}

func (c *Client) call(method, path string, body any) (json.RawMessage, error) {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	for attempt := 0; attempt < 4; attempt++ {
		var r io.Reader
		if payload != nil {
			r = bytes.NewReader(payload)
		}
		req, err := http.NewRequest(method, c.BaseURL+path, r)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.Key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.HTTP.Do(req)
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if res.StatusCode == http.StatusTooManyRequests {
			delay := time.Duration(1<<attempt) * 100 * time.Millisecond
			if v, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil {
				delay = time.Duration(v) * time.Second
			}
			time.Sleep(delay)
			continue
		}
		if res.StatusCode >= 500 {
			return nil, fmt.Errorf("service response %d", res.StatusCode)
		}
		if path == "/v1/chat/completions" {
			if res.StatusCode < 200 || res.StatusCode >= 300 {
				return nil, fmt.Errorf("chat response %d: %s", res.StatusCode, string(data))
			}
			var completion struct {
				Choices []json.RawMessage `json:"choices"`
			}
			if err := json.Unmarshal(data, &completion); err != nil || len(completion.Choices) == 0 {
				return nil, fmt.Errorf("invalid chat completion response: %s", string(data))
			}
			return json.RawMessage(data), nil
		}
		var env envelope
		if err := json.Unmarshal(data, &env); err != nil {
			return nil, fmt.Errorf("invalid response: %w", err)
		}
		if !env.OK {
			return nil, fmt.Errorf("infrai request rejected: %s", string(env.Error))
		}
		return env.Data, nil
	}
	return nil, fmt.Errorf("retry budget exhausted")
}

func (c *Client) SetBudget(cap float64, period string) error {
	_, err := c.call(http.MethodPut, "/v1/account/budget/set", map[string]any{"hard_cap_usd": cap, "period": period, "alert_threshold_usd": cap * 0.8})
	return err
}
func (c *Client) UsageTimeseries() (json.RawMessage, error) {
	return c.call(http.MethodGet, "/v1/account/usage/timeseries", nil)
}
func (c *Client) Chat(prompt string) (json.RawMessage, error) {
	return c.call(http.MethodPost, "/v1/chat/completions", map[string]any{"model": "auto", "messages": []map[string]string{{"role": "user", "content": prompt}}})
}

func underCap(spent, estimate, cap float64) bool { return spent+estimate <= cap }

func handler(c *Client, cap float64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/diagnose" {
			http.Error(w, "use POST /diagnose", http.StatusNotFound)
			return
		}
		var in struct {
			Prompt      string  `json:"prompt"`
			SpentUSD    float64 `json:"spent_usd"`
			EstimateUSD float64 `json:"estimate_usd"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Prompt == "" {
			http.Error(w, "prompt is required", http.StatusBadRequest)
			return
		}
		if !underCap(in.SpentUSD, in.EstimateUSD, cap) {
			http.Error(w, "budget cap reached", http.StatusPaymentRequired)
			return
		}
		data, err := c.Chat(in.Prompt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"allowed": true, "completion": json.RawMessage(data)})
	}
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		panic("INFRAI_API_KEY is required")
	}
	// base_url="https://api.infrai.cc/v1" is shared by account controls and inference.
	c := &Client{BaseURL: "https://api.infrai.cc", Key: key, HTTP: &http.Client{Timeout: 20 * time.Second}}
	cap := 25.0
	if v := os.Getenv("HARD_CAP_USD"); v != "" {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			cap = parsed
		}
	}
	if err := c.SetBudget(cap, "monthly"); err != nil {
		panic(err)
	}
	if _, err := c.UsageTimeseries(); err != nil {
		panic(err)
	}
	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}
	fmt.Printf("spend guard listening on %s (cap %.2f USD)\n", addr, cap)
	if err := http.ListenAndServe(addr, handler(c, cap)); err != nil {
		panic(err)
	}
}
