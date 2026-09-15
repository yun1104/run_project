package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	timestamp := time.Now().Format("20060102_150405")
	username := fmt.Sprintf("e2e_pref_%s", timestamp)
	password := "123456"
	baseURL := "http://127.0.0.1:8080"

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var result string
	var failStep string
	var evidence string

	err := chromedp.Run(ctx,
		chromedp.Navigate(baseURL),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	)
	if err != nil {
		fmt.Printf("FAIL\n步骤: 打开首页\n错误: %v\n", err)
		return
	}
	fmt.Println("✓ 步骤1: 打开 http://127.0.0.1:8080")

	var hasRegisterBtn bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('#registerBtn') !== null`, &hasRegisterBtn),
	)
	if err != nil || !hasRegisterBtn {
		fmt.Printf("FAIL\n步骤: 查找注册按钮\n错误: 未找到注册按钮\n")
		return
	}

	err = chromedp.Run(ctx,
		chromedp.Click(`#registerBtn`, chromedp.ByID),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.WaitVisible(`#registerModal`, chromedp.ByID),
		chromedp.SendKeys(`#regUsername`, username, chromedp.ByID),
		chromedp.SendKeys(`#regPassword`, password, chromedp.ByID),
		chromedp.Click(`#registerSubmit`, chromedp.ByID),
		chromedp.Sleep(1*time.Second),
	)
	if err != nil {
		fmt.Printf("FAIL\n步骤: 注册新用户\n错误: %v\n", err)
		return
	}
	fmt.Printf("✓ 步骤2: 注册新用户 %s\n", username)

	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`#loginBtn`, chromedp.ByID),
		chromedp.Click(`#loginBtn`, chromedp.ByID),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.WaitVisible(`#loginModal`, chromedp.ByID),
		chromedp.SendKeys(`#loginUsername`, username, chromedp.ByID),
		chromedp.SendKeys(`#loginPassword`, password, chromedp.ByID),
		chromedp.Click(`#loginSubmit`, chromedp.ByID),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil {
		fmt.Printf("FAIL\n步骤: 登录\n错误: %v\n", err)
		return
	}
	fmt.Println("✓ 步骤3: 登录成功")

	var prefModalVisible bool
	err = chromedp.Run(ctx,
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`
			(() => {
				const modal = document.querySelector('#prefModal');
				if (!modal) return false;
				const style = window.getComputedStyle(modal);
				return style.display !== 'none' && style.visibility !== 'hidden';
			})()
		`, &prefModalVisible),
	)
	if err != nil {
		fmt.Printf("FAIL\n步骤: 检查偏好问卷弹层\n错误: %v\n", err)
		return
	}

	if !prefModalVisible {
		fmt.Println("FAIL\n步骤: 检查偏好问卷弹层\n错误: 弹层未显示")
		return
	}
	fmt.Println("✓ 步骤4: 偏好问卷弹层已显示")

	var questions []string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			Array.from(document.querySelectorAll('#prefModal .question')).map(q => q.textContent.trim())
		`, &questions),
	)
	if err != nil {
		fmt.Printf("FAIL\n步骤: 获取问题列表\n错误: %v\n", err)
		return
	}
	fmt.Printf("✓ 找到 %d 道问题\n", len(questions))

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const radios = document.querySelectorAll('#prefModal input[type="radio"]');
				const groups = {};
				radios.forEach(radio => {
					if (!groups[radio.name]) {
						groups[radio.name] = radio;
					}
				});
				Object.values(groups).forEach(radio => radio.click());
				return Object.keys(groups).length;
			})()
		`, &result),
		chromedp.Sleep(500*time.Millisecond),
	)
	if err != nil {
		fmt.Printf("FAIL\n步骤: 回答问题\n错误: %v\n", err)
		return
	}
	fmt.Println("✓ 步骤5: 已回答所有问题")

	err = chromedp.Run(ctx,
		chromedp.Click(`#prefSubmit`, chromedp.ByID),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil {
		fmt.Printf("FAIL\n步骤: 提交问卷\n错误: %v\n", err)
		return
	}
	fmt.Println("✓ 步骤6: 提交问卷")

	var modalClosed bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const modal = document.querySelector('#prefModal');
				if (!modal) return true;
				const style = window.getComputedStyle(modal);
				return style.display === 'none' || style.visibility === 'hidden';
			})()
		`, &modalClosed),
	)
	if err != nil {
		fmt.Printf("FAIL\n步骤: 确认弹层关闭\n错误: %v\n", err)
		return
	}

	if !modalClosed {
		fmt.Println("FAIL\n步骤: 确认弹层关闭\n错误: 弹层仍然显示")
		return
	}
	fmt.Println("✓ 步骤7: 弹层已关闭")

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const toast = document.querySelector('.toast, .message, .alert, [role="alert"]');
				if (toast) return toast.textContent.trim();
				const body = document.body.textContent;
				if (body.includes('成功') || body.includes('保存')) {
					return body.substring(0, 200);
				}
				return '未找到明确提示信息';
			})()
		`, &evidence),
	)
	if err != nil {
		evidence = "无法获取页面文本"
	}

	fmt.Println("\n========== 测试结果 ==========")
	fmt.Println("结果: PASS")
	fmt.Printf("用户名: %s\n", username)
	fmt.Printf("关键文本: %s\n", evidence)
	fmt.Println("==============================")
}
