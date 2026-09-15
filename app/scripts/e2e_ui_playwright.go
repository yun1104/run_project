package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

func must(err error, step string) {
	if err != nil {
		fmt.Printf("FAIL %s: %v\n", step, err)
		os.Exit(1)
	}
}

func hasClass(classAttr, cls string) bool {
	for _, c := range strings.Fields(classAttr) {
		if c == cls {
			return true
		}
	}
	return false
}

func main() {
	pw, err := playwright.Run()
	must(err, "playwright.run")
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	must(err, "chromium.launch")
	defer browser.Close()

	page, err := browser.NewPage()
	must(err, "browser.newPage")

	_, err = page.Goto("http://127.0.0.1:8080", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	})
	must(err, "goto home")

	authModal := page.Locator("#authModal")
	className, err := authModal.GetAttribute("class")
	must(err, "read auth modal class")
	if hasClass(className, "hidden") {
		fmt.Println("FAIL step1: auth modal not shown")
		os.Exit(1)
	}
	fmt.Println("PASS step1: auth modal shown")

	ts := time.Now().Unix()
	username := fmt.Sprintf("e2e_ui_%d", ts)
	password := "123456"

	must(page.Click("#authRegisterBtn"), "click register button")
	must(page.Fill("#regUsername", username), "fill reg username")
	must(page.Fill("#regPassword", password), "fill reg password")
	must(page.Fill("#regPassword2", password), "fill reg password2")
	must(page.Click("#regSubmitBtn"), "submit register")
	time.Sleep(800 * time.Millisecond)
	fmt.Println("PASS step2: register submitted")

	must(page.Fill("#authUsername", username), "fill login username")
	must(page.Fill("#authPassword", password), "fill login password")
	must(page.Click("#authLoginBtn"), "click login")

	_, err = page.WaitForFunction(`() => {
	  const el = document.querySelector('#authModal');
	  if (!el) return false;
	  return el.classList.contains('hidden');
	}`, nil)
	must(err, "wait login modal hidden")
	fmt.Println("PASS step3: login success")

	locDeny := page.Locator("#loginLocDenyBtn")
	if visible, _ := locDeny.IsVisible(); visible {
		_ = locDeny.Click()
	}

	must(page.Fill("#promptInput", "预算30元，想吃辣，软件园附近"), "fill prompt")
	must(page.Click("#sendBtn", playwright.PageClickOptions{
		Force: playwright.Bool(true),
	}), "click send")

	_, err = page.WaitForFunction(`() => {
	  const cards = document.querySelectorAll('.cards .card');
	  return cards && cards.length > 0;
	}`, nil, playwright.PageWaitForFunctionOptions{
		Timeout: playwright.Float(15000),
	})
	must(err, "wait recommend cards")
	count, err := page.Locator(".cards .card").Count()
	must(err, "count recommend cards")
	fmt.Printf("PASS step4: recommend cards=%d\n", count)

	_, err = page.Goto("http://127.0.0.1:8080/account", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	})
	must(err, "goto account")
	time.Sleep(500 * time.Millisecond)
	toastText, _ := page.Locator("#toast").TextContent()
	if strings.Contains(toastText, "初始化失败") {
		fmt.Printf("FAIL step5: account init failed, toast=%s\n", toastText)
		os.Exit(1)
	}
	usernameEl := page.Locator("#usernameText")
	usernameVal, _ := usernameEl.TextContent()
	if strings.TrimSpace(usernameVal) == "" || usernameVal == "-" {
		fmt.Printf("FAIL step5: username not loaded, got=%q\n", usernameVal)
		os.Exit(1)
	}
	fmt.Printf("PASS step5: account page ok, username=%s\n", strings.TrimSpace(usernameVal))

	_, err = page.Goto("http://127.0.0.1:8080/assets/location.html", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	})
	must(err, "goto location")
	time.Sleep(500 * time.Millisecond)
	locPermModal := page.Locator("#locationPermModal")
	if visible, _ := locPermModal.IsVisible(); visible {
		_ = page.Locator("#locAllowOnceBtn").Click()
		time.Sleep(2000 * time.Millisecond)
	}
	locToast, _ := page.Locator("#toast").TextContent()
	if strings.Contains(locToast, "初始化失败") {
		fmt.Printf("FAIL step6: location init failed, toast=%s\n", locToast)
		os.Exit(1)
	}
	fmt.Printf("PASS step6: location page ok\n")

	fmt.Println("E2E UI TEST PASSED")
}
