package crawler

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/playwright-community/playwright-go"
)

type MeituanCrawler struct {
	pool     *BrowserPool
	proxyIdx int
	mu       sync.Mutex
}

type BrowserPool struct {
	browsers []*playwright.Browser
	pw       *playwright.Playwright
	mu       sync.Mutex
}

func NewMeituanCrawler(poolSize int) (*MeituanCrawler, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}
	
	pool := &BrowserPool{
		browsers: make([]*playwright.Browser, 0, poolSize),
		pw:       pw,
	}
	
	for i := 0; i < poolSize; i++ {
		browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(true),
		})
		if err != nil {
			return nil, err
		}
		pool.browsers = append(pool.browsers, &browser)
	}
	
	return &MeituanCrawler{
		pool: pool,
	}, nil
}

func (c *MeituanCrawler) GetOrders(userID int64) ([]OrderData, error) {
	browser := c.pool.getBrowser()
	page, err := (*browser).NewPage()
	if err != nil {
		return nil, err
	}
	defer page.Close()
	
	if _, err := page.Goto("https://www.meituan.com/orders", playwright.PageGotoOptions{
		Timeout: playwright.Float(30000),
	}); err != nil {
		return nil, err
	}
	
	time.Sleep(2 * time.Second)
	
	orders := make([]OrderData, 0)
	
	return orders, nil
}

func (c *MeituanCrawler) PlaceOrder(ctx context.Context, order *OrderRequest) error {
	browser := c.pool.getBrowser()
	page, err := (*browser).NewPage()
	if err != nil {
		return err
	}
	defer page.Close()

	if _, err := page.Goto("https://www.meituan.com/merchant/"+order.MerchantID, playwright.PageGotoOptions{
		Timeout: playwright.Float(30000),
	}); err != nil {
		return err
	}

	time.Sleep(1 * time.Second)

	return nil
}

// SearchStore 在美团美食首页模拟搜索框输入并搜索，返回匹配店铺或搜索结果页 URL
func (c *MeituanCrawler) SearchStore(ctx context.Context, merchantName string) (storeURL, searchURL string, found bool, err error) {
	if strings.TrimSpace(merchantName) == "" {
		return "", "", false, nil
	}
	browser := c.pool.getBrowser()
	if browser == nil {
		return "", "", false, nil
	}
	page, err := (*browser).NewPage()
	if err != nil {
		return "", "", false, err
	}
	defer page.Close()

	// 设置视口，降低被识别为 headless 的概率
	_ = page.SetViewportSize(1280, 720)

	baseURL := "https://meishi.meituan.com/i/"
	if _, err := page.Goto(baseURL, playwright.PageGotoOptions{
		Timeout:   playwright.Float(20000),
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return "", "", false, err
	}
	// SPA 渲染可能较慢，显式等待搜索框
	time.Sleep(3 * time.Second)

	// 组合选择器：placeholder 含「商家」「品类」「商圈」或「输入」的 input
	searchSelectors := []string{
		`input[placeholder*="商家"]`,
		`input[placeholder*="品类"]`,
		`input[placeholder*="商圈"]`,
		`input[placeholder*="输入"]`,
		`input[type="search"]`,
		`[class*="search"] input`,
		`input.search-input`,
	}
	var searchFound bool
	for _, sel := range searchSelectors {
		loc := page.Locator(sel).First()
		// 等待元素可见（Locator 内置超时）
		if waitErr := loc.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(8000)}); waitErr != nil {
			continue
		}
		// 点击聚焦后填充，模拟真实输入
		if clickErr := loc.Click(); clickErr != nil {
			continue
		}
		time.Sleep(500 * time.Millisecond)
		if fillErr := loc.Fill(merchantName); fillErr != nil {
			continue
		}
		time.Sleep(300 * time.Millisecond)
		if pressErr := loc.Press("Enter"); pressErr != nil {
			continue
		}
		searchFound = true
		break
	}
	if !searchFound {
		encoded := url.QueryEscape(strings.TrimSpace(merchantName))
		if _, goErr := page.Goto(baseURL+"?keyword="+encoded, playwright.PageGotoOptions{
			Timeout: playwright.Float(15000),
		}); goErr == nil {
			searchFound = true
		}
	}
	// 等待搜索结果加载
	time.Sleep(6 * time.Second)

	currentURL := page.URL()
	if currentURL == "" {
		currentURL = baseURL + "?keyword=" + url.QueryEscape(strings.TrimSpace(merchantName))
	}

	links, _ := page.Locator(`a[href*="/poi/"]`).All()
	if len(links) > 5 {
		links = links[:5]
	}

	normalizedQuery := normalizeMerchantName(merchantName)
	for i := 0; i < len(links); i++ {
		href, _ := links[i].GetAttribute("href")
		if href == "" {
			continue
		}
		fullURL := href
		if !strings.HasPrefix(href, "http") {
			fullURL = "https://meishi.meituan.com" + href
		}
		text, _ := links[i].TextContent()
		if matchMerchantName(normalizedQuery, text) {
			return fullURL, "", true, nil
		}
	}
	return "", currentURL, false, nil
}

func normalizeMerchantName(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		if unicode.IsSpace(r) || r == '·' || r == '．' {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func matchMerchantName(query, result string) bool {
	if result == "" {
		return false
	}
	q := normalizeMerchantName(query)
	r := normalizeMerchantName(result)
	if strings.Contains(r, q) || strings.Contains(q, r) {
		return true
	}
	// 简单子串匹配：query 的主要部分在 result 中
	words := strings.FieldsFunc(q, func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	})
	for _, w := range words {
		if len(w) >= 2 && strings.Contains(r, w) {
			return true
		}
	}
	return false
}

func (c *MeituanCrawler) Close() {
	for _, browser := range c.pool.browsers {
		(*browser).Close()
	}
	c.pool.pw.Stop()
}

func (p *BrowserPool) getBrowser() *playwright.Browser {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if len(p.browsers) == 0 {
		return nil
	}
	
	browser := p.browsers[0]
	p.browsers = append(p.browsers[1:], browser)
	
	return browser
}

type OrderData struct {
	OrderID      int64
	MerchantID   int64
	MerchantName string
	Dishes       []string
	TotalPrice   float64
	OrderTime    string
}

type OrderRequest struct {
	MerchantID string
	Dishes     []DishItem
	Address    string
}

type DishItem struct {
	DishID   string
	Quantity int
}
