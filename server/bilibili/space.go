package bilibili

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"
)

// SpaceVideo holds the fields we extract from a UP's video list.
type SpaceVideo struct {
	Aid      int64  `json:"aid"`
	Bvid     string `json:"bvid"`
	Title    string `json:"title"`
	Pic      string `json:"pic"`
	Duration int    `json:"duration"`
	Pubdate  int64  `json:"pubdate"`
	Play     int64  `json:"play"`
	Danmaku  int64  `json:"danmaku"`
}

type spaceCacheEntry struct {
	videos []SpaceVideo
	ts     time.Time
}

var (
	spaceCache    = make(map[int64]*spaceCacheEntry)
	spaceCacheMu  sync.RWMutex
	spaceCacheTTL = 5 * time.Minute
)

const (
	biliSpaceAPI = "https://app.bilibili.com/x/v2/space/archive/cursor"
	appUA        = "Mozilla/5.0 BiliDroid/8.43.0 (bbcallen@gmail.com) os/android model/android mobi_app/android build/8430300 channel/master innerVer/8430300 osVer/15 network/2"
	appStats     = `{"appId":1,"platform":3,"version":"8.43.0","abtest":""}`

	// App API signing credentials (same as the app uses)
	appKey = "dfca71928277209b"
	appSec = "b5475a8825547a4fc26c7d518eaaa02e"
)

// appSign adds appkey, ts, and sign to the params (Bilibili app API auth).
// Algorithm: sort params by key → build query string → MD5(query + appsec).
func appSign(params url.Values) {
	params.Set("appkey", appKey)
	params.Set("ts", fmt.Sprintf("%d", time.Now().Unix()))

	// Sort keys
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build sorted query string
	query := ""
	for i, k := range keys {
		if i > 0 {
			query += "&"
		}
		query += url.QueryEscape(k) + "=" + url.QueryEscape(params.Get(k))
	}

	// sign = MD5(query + appsec)
	hash := md5.Sum([]byte(query + appSec))
	params.Set("sign", hex.EncodeToString(hash[:]))
}

// FetchUserVideos queries Bilibili's App API for a UP's recent videos.
// Results are cached per mid with a 5-minute TTL.
func FetchUserVideos(mid int64, ps int) ([]SpaceVideo, error) {
	// Check cache
	spaceCacheMu.RLock()
	if entry, ok := spaceCache[mid]; ok && time.Since(entry.ts) < spaceCacheTTL {
		spaceCacheMu.RUnlock()
		return entry.videos, nil
	}
	spaceCacheMu.RUnlock()

	params := url.Values{
		"vmid":       {fmt.Sprintf("%d", mid)},
		"ps":         {fmt.Sprintf("%d", ps)},
		"order":      {"pubdate"},
		"qn":         {"80"},
		"build":      {"8430300"},
		"version":    {"8.43.0"},
		"mobi_app":   {"android"},
		"platform":   {"android"},
		"channel":    {"master"},
		"c_locale":   {"zh_CN"},
		"s_locale":   {"zh_CN"},
		"statistics": {appStats},
	}
	appSign(params)

	reqURL := biliSpaceAPI + "?" + params.Encode()

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", appUA)
	req.Header.Set("Referer", "https://www.bilibili.com")
	req.Header.Set("bili-http-engine", "cronet")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[space] FetchUserVideos mid=%d network error: %v", mid, err)
		return nil, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[space] FetchUserVideos mid=%d read body error: %v", mid, err)
		return nil, nil
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Item []struct {
				Param    string `json:"param"`
				Bvid     string `json:"bvid"`
				Title    string `json:"title"`
				Cover    string `json:"cover"`
				Duration int    `json:"duration"`
				Ctime    int64  `json:"ctime"`
				Play     int64  `json:"play"`
				Danmaku  int64  `json:"danmaku"`
			} `json:"item"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("[space] FetchUserVideos mid=%d JSON parse error: %v", mid, err)
		return nil, nil
	}

	if result.Code != 0 {
		log.Printf("[space] FetchUserVideos mid=%d API error: code=%d msg=%s", mid, result.Code, result.Message)
		spaceCacheMu.Lock()
		spaceCache[mid] = &spaceCacheEntry{videos: nil, ts: time.Now().Add(-spaceCacheTTL + time.Minute)}
		spaceCacheMu.Unlock()
		return nil, nil
	}

	videos := make([]SpaceVideo, 0, len(result.Data.Item))
	for _, v := range result.Data.Item {
		var aid int64
		fmt.Sscanf(v.Param, "%d", &aid)
		videos = append(videos, SpaceVideo{
			Aid:      aid,
			Bvid:     v.Bvid,
			Title:    v.Title,
			Pic:      v.Cover,
			Duration: v.Duration,
			Pubdate:  v.Ctime,
			Play:     v.Play,
			Danmaku:  v.Danmaku,
		})
	}

	log.Printf("[space] FetchUserVideos mid=%d fetched %d videos", mid, len(videos))

	spaceCacheMu.Lock()
	spaceCache[mid] = &spaceCacheEntry{videos: videos, ts: time.Now()}
	spaceCacheMu.Unlock()

	return videos, nil
}
