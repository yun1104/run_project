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
    username = f"e2e_intent_{timestamp}"
    password = "123456"
    base_url = "http://127.0.0.1:8080"
    
    chrome_options = Options()
    chrome_options.add_argument('--disable-gpu')
    chrome_options.add_argument('--no-sandbox')
    chrome_options.add_argument('--disable-dev-shm-usage')
    chrome_options.add_argument('--disable-blink-features=AutomationControlled')
    chrome_options.add_experimental_option('excludeSwitches', ['enable-logging'])
    
    driver = None
    try:
        driver = webdriver.Chrome(options=chrome_options)
        driver.maximize_window()
        driver.set_page_load_timeout(30)
        wait = WebDriverWait(driver, 10)
        
        print(f"步骤1: 打开 {base_url}")
        driver.get(base_url)
        time.sleep(1)
        
        print(f"步骤2: 注册新用户 {username}")
        try:
            register_btn = wait.until(EC.element_to_be_clickable((By.ID, "authRegisterBtn")))
        except:
            driver.find_element(By.ID, "sendBtn").click()
            time.sleep(0.5)
            register_btn = wait.until(EC.element_to_be_clickable((By.ID, "authRegisterBtn")))
        register_btn.click()
        time.sleep(0.5)
        
        wait.until(EC.visibility_of_element_located((By.ID, "registerModal")))
        driver.find_element(By.ID, "regUsername").send_keys(username)
        driver.find_element(By.ID, "regPassword").send_keys(password)
        driver.find_element(By.ID, "regPassword2").send_keys(password)
        driver.find_element(By.ID, "regSubmitBtn").click()
        time.sleep(2)
        
        print("步骤3: 登录")
        wait.until(EC.visibility_of_element_located((By.ID, "authModal")))
        time.sleep(0.5)
        
        auth_username_input = driver.find_element(By.ID, "authUsername")
        auth_password_input = driver.find_element(By.ID, "authPassword")
        
        driver.execute_script("arguments[0].value = ''", auth_username_input)
        driver.execute_script("arguments[0].value = ''", auth_password_input)
        time.sleep(0.2)
        
        auth_username_input.send_keys(username)
        auth_password_input.send_keys(password)
        login_btn = driver.find_element(By.ID, "authLoginBtn")
        
        driver.execute_script("""
            window.loginSuccess = false;
            const oldSetAuth = window.setAuth;
            window.setAuth = function(data) {
                if (oldSetAuth) oldSetAuth(data);
                window.loginSuccess = true;
            };
        """)
        
        login_btn.click()
        
        for i in range(30):
            time.sleep(0.5)
            login_done = driver.execute_script("return window.loginSuccess === true")
            if login_done:
                break
        
        time.sleep(1)
        
        print("步骤4: 处理定位授权和偏好问卷")
        time.sleep(2)
        
        try:
            loc_modal = driver.find_element(By.ID, "loginLocationPermModal")
            is_visible = driver.execute_script("""
                const modal = arguments[0];
                return !modal.classList.contains('hidden');
            """, loc_modal)
            
            if is_visible:
                driver.find_element(By.ID, "loginLocDenyBtn").click()
                time.sleep(1.5)
        except:
            pass
        
        try:
            pref_modal = driver.find_element(By.ID, "prefModal")
            is_visible = driver.execute_script("""
                const modal = arguments[0];
                return !modal.classList.contains('hidden');
            """, pref_modal)
            
            if is_visible:
                for _ in range(10):
                    driver.execute_script("""
                        const radios = document.querySelectorAll('#prefModal input[type="radio"]');
                        if (radios.length > 0) radios[0].click();
                    """)
                    time.sleep(0.3)
                    try:
                        next_btn = driver.find_element(By.ID, "prefNextBtn")
                        if next_btn.text == "完成":
                            next_btn.click()
                            break
                        next_btn.click()
                    except:
                        break
                time.sleep(2)
        except:
            pass
        
        print("步骤5: 发送普通聊天消息'你好'")
        chat_input = wait.until(EC.presence_of_element_located((By.ID, "promptInput")))
        
        driver.execute_script("arguments[0].scrollIntoView(true);", chat_input)
        driver.execute_script("arguments[0].value = '你好'", chat_input)
        
        driver.execute_script("document.getElementById('sendBtn').click()")
        time.sleep(4)
        
        cards_before = driver.execute_script("""
            return document.querySelectorAll('.cards .card').length;
        """)
        
        reply1 = driver.execute_script("""
            const messages = document.querySelectorAll('.message, .chat-message, .msg');
            if (messages.length > 0) {
                return messages[messages.length - 1].textContent.trim();
            }
            return '';
        """)
        
        print(f"  - 助手回复: {reply1[:100]}")
        print(f"  - 卡片数量: {cards_before}")
        
        print("步骤6: 发送推荐意图消息")
        driver.execute_script("document.getElementById('promptInput').value = '预算30元，想吃辣，30分钟送达'")
        time.sleep(0.3)
        
        driver.execute_script("document.getElementById('sendBtn').click()")
        time.sleep(6)
        
        cards_after = driver.execute_script("""
            return document.querySelectorAll('.cards .card').length;
        """)
        
        reply2 = driver.execute_script("""
            const messages = document.querySelectorAll('.message, .chat-message, .msg');
            if (messages.length > 0) {
                return messages[messages.length - 1].textContent.trim();
            }
            return '';
        """)
        
        print(f"  - 助手回复: {reply2[:100]}")
        print(f"  - 卡片数量: {cards_after}")
        
        print("\n========== 验证结果 ==========")
        
        pass_checks = []
        fail_checks = []
        
        if cards_before == 0:
            pass_checks.append("✓ 普通聊天无推荐卡片")
        else:
            fail_checks.append(f"✗ 普通聊天出现了{cards_before}张卡片")
        
        if cards_after >= 1:
            pass_checks.append(f"✓ 推荐意图生成了{cards_after}张卡片")
        else:
            fail_checks.append("✗ 推荐意图未生成卡片")
        
        if len(fail_checks) == 0:
            print("结果: PASS")
        else:
            print("结果: FAIL")
        
        print(f"\n用户名: {username}")
        print(f"普通聊天回复: {reply1[:200]}")
        print(f"推荐意图回复: {reply2[:200]}")
        print(f"普通聊天卡片数: {cards_before}")
        print(f"推荐意图卡片数: {cards_after}")
        
        for check in pass_checks:
            print(check)
        for check in fail_checks:
            print(check)
        
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
