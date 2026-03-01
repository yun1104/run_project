package crawler

import (
	"context"
	"github.com/playwright-community/playwright-go"
	"sync"
	"time"
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
