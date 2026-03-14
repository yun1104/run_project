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
		fmt.Printf("FAIL\n步骤: %s\n错误: %v\n", step, err)
		os.Exit(1)
	}
}

func main() {
	pw, err := playwright.Run()
	must(err, "启动playwright")
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	must(err, "启动浏览器")
	defer browser.Close()

	page, err := browser.NewPage()
	must(err, "创建页面")

	ts := time.Now().Unix()
	username := fmt.Sprintf("e2e_pref_%d", ts)
	password := "123456"

	fmt.Println("✓ 步骤1: 打开 http://127.0.0.1:8080")
	_, err = page.Goto("http://127.0.0.1:8080", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	})
	must(err, "打开首页")

	fmt.Printf("✓ 步骤2: 注册新用户 %s\n", username)
	registerBtn := page.Locator("#authRegisterBtn")
	must(registerBtn.Click(), "点击注册按钮")
	time.Sleep(500 * time.Millisecond)

	must(page.Fill("#regUsername", username), "填写注册用户名")
	must(page.Fill("#regPassword", password), "填写注册密码")
	must(page.Fill("#regPassword2", password), "填写确认密码")
	must(page.Click("#regSubmitBtn"), "提交注册")
	time.Sleep(1 * time.Second)

	fmt.Println("✓ 步骤3: 登录")
	must(page.Fill("#authUsername", username), "填写登录用户名")
	must(page.Fill("#authPassword", password), "填写登录密码")
	must(page.Click("#authLoginBtn"), "提交登录")
	
	_, err = page.WaitForFunction(`() => {
		return document.querySelector('#authModal').classList.contains('hidden');
	}`, nil, playwright.PageWaitForFunctionOptions{
		Timeout: playwright.Float(5000),
	})
	must(err, "等待登录完成")
	
	time.Sleep(1 * time.Second)
	
	_, err = page.Evaluate(`async () => {
		await initPreferenceOnFirstUse();
	}`)
	must(err, "触发偏好问卷初始化")
	
	time.Sleep(1 * time.Second)

	fmt.Println("✓ 步骤4: 检查偏好问卷弹层")
	
	_, err = page.WaitForFunction(`() => {
		const modal = document.querySelector('#prefModal');
		if (!modal) return false;
		return !modal.classList.contains('hidden');
	}`, nil, playwright.PageWaitForFunctionOptions{
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		fmt.Println("FAIL")
		fmt.Println("步骤: 检查偏好问卷弹层")
		fmt.Printf("错误: 弹层未在10秒内显示: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ 偏好问卷弹层已显示")

	fmt.Println("✓ 步骤5: 回答所有问题")
	
	for i := 0; i < 6; i++ {
		time.Sleep(500 * time.Millisecond)
		
		_, err = page.Evaluate(`() => {
			const buttons = document.querySelectorAll('#prefOptions button');
			if (buttons.length > 0) buttons[0].click();
		}`)
		must(err, fmt.Sprintf("回答第%d题", i+1))
		time.Sleep(300 * time.Millisecond)
		
		_, err = page.Evaluate(`() => document.querySelector('#prefNextBtn').click()`)
		must(err, fmt.Sprintf("点击第%d题的下一步/提交", i+1))
		time.Sleep(500 * time.Millisecond)
	}
	
	fmt.Println("✓ 步骤6: 提交问卷")
	time.Sleep(2 * time.Second)

	fmt.Println("✓ 步骤7: 确认弹层关闭")
	modalClosed, err := page.Evaluate(`() => {
		const modal = document.querySelector('#prefModal');
		if (!modal) return true;
		const style = window.getComputedStyle(modal);
		return style.display === 'none' || style.visibility === 'hidden';
	}`)
	must(err, "检查弹层关闭")

	if closed, ok := modalClosed.(bool); !ok || !closed {
		fmt.Println("FAIL")
		fmt.Println("步骤: 确认弹层关闭")
		fmt.Println("错误: 弹层仍然显示")
		os.Exit(1)
	}

	fmt.Println("✓ 弹层已关闭")

	evidence, err := page.Evaluate(`() => {
		const toast = document.querySelector('.toast, .message, .alert, [role="alert"]');
		if (toast) return toast.textContent.trim();
		const body = document.body.textContent;
		if (body.includes('成功') || body.includes('保存')) {
			return body.substring(0, 200);
		}
		return '未找到明确提示信息';
	}`)
	if err != nil {
		evidence = "无法获取页面文本"
	}

	evidenceStr := ""
	if str, ok := evidence.(string); ok {
		evidenceStr = strings.TrimSpace(str)
	}

	fmt.Println("\n========== 测试结果 ==========")
	fmt.Println("结果: PASS")
	fmt.Printf("用户名: %s\n", username)
	fmt.Printf("关键文本: %s\n", evidenceStr)
	fmt.Println("==============================")
}
