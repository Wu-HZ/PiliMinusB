package bilibili

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// VideoInfo holds the fields we extract from Bilibili's /x/web-interface/view API.
type VideoInfo struct {
	Aid       int64  `json:"aid"`
	Bvid      string `json:"bvid"`
	Title     string `json:"title"`
	Pic       string `json:"pic"`
	Duration  int    `json:"duration"`
	Pubdate   int64  `json:"pubdate"`
	Cid       int64  `json:"cid"`
	Videos    int    `json:"videos"`
	OwnerMid  int64
	OwnerName string
	OwnerFace string
}

var (
	cache   = make(map[string]*VideoInfo)
	cacheMu sync.RWMutex
)

const biliViewAPI = "https://api.bilibili.com/x/web-interface/view"

// FetchVideoInfo queries Bilibili's public API for video metadata.
// Results are cached in memory to avoid repeated requests.
func FetchVideoInfo(aid int64, bvid string) (*VideoInfo, error) {
	// Build cache key
	key := bvid
	if key == "" {
		key = fmt.Sprintf("av%d", aid)
	}

	// Check cache
	cacheMu.RLock()
	if info, ok := cache[key]; ok {
		cacheMu.RUnlock()
		return info, nil
	}
	cacheMu.RUnlock()

	// Build request URL
	var url string
	if bvid != "" {
		url = fmt.Sprintf("%s?bvid=%s", biliViewAPI, bvid)
	} else {
		url = fmt.Sprintf("%s?aid=%d", biliViewAPI, aid)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bilibili API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Aid      int64  `json:"aid"`
			Bvid     string `json:"bvid"`
			Title    string `json:"title"`
			Pic      string `json:"pic"`
			Duration int    `json:"duration"`
			Pubdate  int64  `json:"pubdate"`
			Cid      int64  `json:"cid"`
			Videos   int    `json:"videos"`
			Owner    struct {
				Mid  int64  `json:"mid"`
				Name string `json:"name"`
				Face string `json:"face"`
			} `json:"owner"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Code != 0 {
		// Video may be deleted/unavailable — return a placeholder
		info := &VideoInfo{
			Aid:   aid,
			Bvid:  bvid,
			Title: "已失效视频",
		}
		cacheMu.Lock()
		cache[key] = info
		cacheMu.Unlock()
		return info, nil
	}

	info := &VideoInfo{
		Aid:       result.Data.Aid,
		Bvid:      result.Data.Bvid,
		Title:     result.Data.Title,
		Pic:       result.Data.Pic,
		Duration:  result.Data.Duration,
		Pubdate:   result.Data.Pubdate,
		Cid:       result.Data.Cid,
		Videos:    result.Data.Videos,
		OwnerMid:  result.Data.Owner.Mid,
		OwnerName: result.Data.Owner.Name,
		OwnerFace: result.Data.Owner.Face,
	}

	cacheMu.Lock()
	cache[key] = info
	cacheMu.Unlock()

	return info, nil
}
