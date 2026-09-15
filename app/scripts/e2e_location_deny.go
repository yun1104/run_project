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
		fmt.Printf("FAIL|%s|%v\n", step, err)
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
	must(err, "step1_goto")

	authModal := page.Locator("#authModal")
	className, err := authModal.GetAttribute("class")
	must(err, "step1_check_modal")
	if hasClass(className, "hidden") {
		fmt.Println("FAIL|step1|auth modal not shown")
		os.Exit(1)
	}

	ts := time.Now().Unix()
	username := fmt.Sprintf("e2e_locdeny_%d", ts)
	password := "123456"

	must(page.Click("#authRegisterBtn"), "step2_click_register")
	must(page.Fill("#regUsername", username), "step2_fill_username")
	must(page.Fill("#regPassword", password), "step2_fill_password")
	must(page.Fill("#regPassword2", password), "step2_fill_password2")
	must(page.Click("#regSubmitBtn"), "step2_submit_register")
	time.Sleep(800 * time.Millisecond)

	must(page.Fill("#authUsername", username), "step2_fill_login_username")
	must(page.Fill("#authPassword", password), "step2_fill_login_password")
	must(page.Click("#authLoginBtn"), "step2_click_login")

	_, err = page.WaitForFunction(`() => {
	  const el = document.querySelector('#authModal');
	  if (!el) return false;
	  return el.classList.contains('hidden');
	}`, nil)
	must(err, "step2_wait_login_success")

	locDeny := page.Locator("#loginLocDenyBtn")
	if visible, _ := locDeny.IsVisible(); visible {
		must(locDeny.Click(), "step3_click_deny_login_location")
	}
	time.Sleep(500 * time.Millisecond)

	_, err = page.Goto("http://127.0.0.1:8080/assets/location.html", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	})
	must(err, "step4_goto_location")
	time.Sleep(500 * time.Millisecond)

	locPermModal := page.Locator("#locationPermModal")
	if visible, _ := locPermModal.IsVisible(); visible {
		denyBtn := page.Locator("#locDenyBtn")
		must(denyBtn.Click(), "step5_click_deny_location_perm")
		time.Sleep(1000 * time.Millisecond)
	}

	currentLocDiv := page.Locator("#locationSummary")
	currentLocText, err := currentLocDiv.TextContent()
	must(err, "step6_read_current_location")
	currentLocText = strings.TrimSpace(currentLocText)

	if !strings.Contains(currentLocText, "未授权") && !strings.Contains(currentLocText, "无法查询") {
		fmt.Printf("FAIL|step6|expected unauthorized text, got: %s\n", currentLocText)
		os.Exit(1)
	}

	if strings.Contains(currentLocText, "纬度") || strings.Contains(currentLocText, "经度") {
		fmt.Printf("FAIL|step6|should not show coordinates, got: %s\n", currentLocText)
		os.Exit(1)
	}

	nearbyDiv := page.Locator("#nearbyList")
	nearbyText, err := nearbyDiv.TextContent()
	must(err, "step6_read_nearby")
	nearbyText = strings.TrimSpace(nearbyText)

	refreshBtn := page.Locator("#refreshLocationBtn")
	must(refreshBtn.Click(), "step7_click_refresh")
	time.Sleep(2000 * time.Millisecond)

	currentLocText2, err := currentLocDiv.TextContent()
	must(err, "step7_read_current_location_after_refresh")
	currentLocText2 = strings.TrimSpace(currentLocText2)

	if strings.Contains(currentLocText2, "纬度") || strings.Contains(currentLocText2, "经度") {
		fmt.Printf("FAIL|step7|should not show coordinates after refresh, got: %s\n", currentLocText2)
		os.Exit(1)
	}

	fmt.Printf("PASS|step6_current_location|%s\n", currentLocText)
	fmt.Printf("PASS|step6_nearby_restaurants|%s\n", nearbyText)
	fmt.Printf("PASS|step7_after_refresh|%s\n", currentLocText2)
	fmt.Println("PASS|all_steps|location deny test completed")
}
