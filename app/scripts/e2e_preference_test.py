#!/usr/bin/env python3
# -*- coding: utf-8 -*-
import time
from datetime import datetime
from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC
from selenium.webdriver.chrome.options import Options

def main():
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    username = f"e2e_pref_{timestamp}"
    password = "123456"
    base_url = "http://127.0.0.1:8080"
    
    chrome_options = Options()
    chrome_options.add_argument('--headless')
    chrome_options.add_argument('--disable-gpu')
    chrome_options.add_argument('--no-sandbox')
    chrome_options.add_argument('--disable-dev-shm-usage')
    
    driver = None
    try:
        driver = webdriver.Chrome(options=chrome_options)
        driver.set_page_load_timeout(30)
        wait = WebDriverWait(driver, 10)
        
        print(f"✓ 步骤1: 打开 {base_url}")
        driver.get(base_url)
        time.sleep(1)
        
        print(f"✓ 步骤2: 注册新用户 {username}")
        register_btn = wait.until(EC.element_to_be_clickable((By.ID, "registerBtn")))
        register_btn.click()
        time.sleep(0.5)
        
        wait.until(EC.visibility_of_element_located((By.ID, "registerModal")))
        driver.find_element(By.ID, "regUsername").send_keys(username)
        driver.find_element(By.ID, "regPassword").send_keys(password)
        driver.find_element(By.ID, "registerSubmit").click()
        time.sleep(1)
        
        print("✓ 步骤3: 登录")
        login_btn = wait.until(EC.element_to_be_clickable((By.ID, "loginBtn")))
        login_btn.click()
        time.sleep(0.5)
        
        wait.until(EC.visibility_of_element_located((By.ID, "loginModal")))
        driver.find_element(By.ID, "loginUsername").send_keys(username)
        driver.find_element(By.ID, "loginPassword").send_keys(password)
        driver.find_element(By.ID, "loginSubmit").click()
        time.sleep(2)
        
        print("✓ 步骤4: 检查偏好问卷弹层")
        pref_modal = wait.until(EC.presence_of_element_located((By.ID, "prefModal")))
        is_visible = driver.execute_script("""
            const modal = arguments[0];
            const style = window.getComputedStyle(modal);
            return style.display !== 'none' && style.visibility !== 'hidden';
        """, pref_modal)
        
        if not is_visible:
            print("FAIL")
            print("步骤: 检查偏好问卷弹层")
            print("错误: 弹层未显示")
            return
        
        print("✓ 偏好问卷弹层已显示")
        
        questions = driver.execute_script("""
            return Array.from(document.querySelectorAll('#prefModal .question')).map(q => q.textContent.trim());
        """)
        print(f"✓ 找到 {len(questions)} 道问题")
        
        print("✓ 步骤5: 回答所有问题")
        driver.execute_script("""
            const radios = document.querySelectorAll('#prefModal input[type="radio"]');
            const groups = {};
            radios.forEach(radio => {
                if (!groups[radio.name]) {
                    groups[radio.name] = radio;
                }
            });
            Object.values(groups).forEach(radio => radio.click());
        """)
        time.sleep(0.5)
        
        print("✓ 步骤6: 提交问卷")
        submit_btn = driver.find_element(By.ID, "prefSubmit")
        submit_btn.click()
        time.sleep(2)
        
        print("✓ 步骤7: 确认弹层关闭")
        modal_closed = driver.execute_script("""
            const modal = document.querySelector('#prefModal');
            if (!modal) return true;
            const style = window.getComputedStyle(modal);
            return style.display === 'none' || style.visibility === 'hidden';
        """)
        
        if not modal_closed:
            print("FAIL")
            print("步骤: 确认弹层关闭")
            print("错误: 弹层仍然显示")
            return
        
        print("✓ 弹层已关闭")
        
        evidence = driver.execute_script("""
            const toast = document.querySelector('.toast, .message, .alert, [role="alert"]');
            if (toast) return toast.textContent.trim();
            const body = document.body.textContent;
            if (body.includes('成功') || body.includes('保存')) {
                return body.substring(0, 200);
            }
            return '未找到明确提示信息';
        """)
        
        print("\n========== 测试结果 ==========")
        print("结果: PASS")
        print(f"用户名: {username}")
        print(f"关键文本: {evidence}")
        print("==============================")
        
    except Exception as e:
        print("\nFAIL")
        print(f"错误: {str(e)}")
        if driver:
            print(f"当前URL: {driver.current_url}")
    finally:
        if driver:
            driver.quit()

if __name__ == "__main__":
    main()
