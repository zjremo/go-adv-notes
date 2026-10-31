package client_test

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	dotenv "github.com/joho/godotenv"
	jsoniter "github.com/json-iterator/go"
)

func TestGet(t *testing.T) {
	base_url := "https://api-example.com/users"

	req, err := http.NewRequest(http.MethodGet, base_url, nil)
	if err != nil {
		t.Fatal("NewRequest failed, err:", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal("call req failed, err:", err)
	}
	defer resp.Body.Close()

	respBody := make([]map[string]any, 0)
	if err := jsoniter.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		t.Fatal("decode respBody failed, err:", err)
	}

	for idx, m := range respBody {
		t.Logf("id%d: %#v\n", idx, m)
	}
}

func TestPost(t *testing.T) {
	// 1. load env variables
	if err := dotenv.Load("../.env"); err != nil {
		t.Fatal("load env failed, err:", err)
	}

	// 2. check env load
	base_url := os.Getenv("DEEPSEEK_BASE_URL")
	if len(base_url) == 0 {
		t.Fatal("get DEEPSEEK_BASE_URL failed")
	}

	api_key := os.Getenv("DEEPSEEK_API_KEY")
	if len(api_key) == 0 {
		t.Fatal("get DEEPSEEK_API_KEY failed")
	}

	// 3. build request
	// 3.1 build reqBody
	reqBody := map[string]any{
		"query":        "DeepSeek V4 模型 发布",
		"count":        5,
		"need_content": true,
	}
	jsonBody, err := jsoniter.MarshalToString(reqBody)
	if err != nil {
		t.Fatal("jsoniter MarshalToString failed, reqBody:", reqBody)
	}

	req, err := http.NewRequest(http.MethodPost, base_url, strings.NewReader(jsonBody))
	if err != nil {
		t.Fatal("NewRequest failed, err:", err)
	}

	// 3.2 build req Header
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", api_key))

	// 4. create client to call the api
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal("call req failed, err:", err)
	}
	defer resp.Body.Close()

	// 5. get respBody
	// 5.1 decode
	respBody := make(map[string]any, 0)
	if err = jsoniter.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		t.Fatal("decode respBody failed, err:", err)
	}

	// 5.2 analysis
	results, ok := respBody["results"].([]any)
	if !ok {
		t.Fatal("get results from respBody failed!, err:", err)
	}

	for idx, result := range results {
		t.Log("------------idx:", idx, "------------")
		resMap, ok := result.(map[string]any)
		if !ok {
			t.Fatal("result type is not map[string]any")
		}
		title := resMap["title"]
		url := resMap["url"]
		summary := resMap["summary"]
		t.Logf("title:%s\nurl:%s\nsummary:%s\n", title, url, summary)
	}
}
