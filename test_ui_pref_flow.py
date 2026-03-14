#!/usr/bin/env python3
# -*- coding: utf-8 -*-
import time
from datetime import datetime
from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC
from selenium.webdriver.chrome.options import Options

def test_preference_flow():
    chrome_options = Options()
    chrome_options.add_argument('--headless')
    chrome_options.add_argument('--disable-gpu')
    chrome_options.add_argument('--no-sandbox')
    chrome_options.add_argument('--disable-dev-shm-usage')
    
    driver = webdriver.Chrome(options=chrome_options)
    wait = WebDriverWait(driver, 10)
    
    results = []
    final_status = "PASS"
    
    try:
        timestamp = datetime.now().strftime("%Y%m%d%H%M%S")
        username = f"e2e_pref_once_{timestamp}"
        password = "123456"
        
        # Step 1: 打开首页
        driver.get("http://127.0.0.1:8080")
        time.sleep(1)
        results.append(f"[STEP 1] 打开首页: {driver.title}")
        
        # Step 2: 注册新用户
        register_btn = wait.until(EC.element_to_be_clickable((By.ID, "showRegister")))
        register_btn.click()
        time.sleep(0.5)
        
        driver.find_element(By.ID, "regUsername").send_keys(username)
        driver.find_element(By.ID, "regPassword").send_keys(password)
        driver.find_element(By.CSS_SELECTOR, "#registerModal button[type='submit']").click()
        time.sleep(1)
        
        alert_text = driver.find_element(By.ID, "alertMessage").text
        results.append(f"[STEP 2] 注册用户 {username}: {alert_text}")
        
        # 登录
        driver.find_element(By.ID, "username").send_keys(username)
        driver.find_element(By.ID, "password").send_keys(password)
        driver.find_element(By.CSS_SELECTOR, "form button[type='submit']").click()
        time.sleep(2)
        
        # Step 3: 处理定位授权弹层
        try:
            deny_btn = driver.find_element(By.ID, "denyLocation")
            if deny_btn.is_displayed():
                deny_btn.click()
                time.sleep(0.5)
                results.append("[STEP 3] 点击定位授权'不允许'")
        except:
            results.append("[STEP 3] 未出现定位授权弹层")
        
        # Step 4: 验证偏好问卷弹层出现
        time.sleep(1)
        pref_modal = driver.find_element(By.ID, "prefModal")
        is_visible = pref_modal.is_displayed()
        
        if is_visible:
            modal_title = driver.find_element(By.CSS_SELECTOR, "#prefModal .modal-title").text
            results.append(f"[STEP 4] 偏好问卷弹层出现: '{modal_title}'")
        else:
            results.append("[STEP 4] FAIL: 偏好问卷弹层未出现")
            final_status = "FAIL"
            return
        
        # Step 5: 完成问卷并提交
        driver.find_element(By.CSS_SELECTOR, "input[name='cuisine'][value='川菜']").click()
        driver.find_element(By.CSS_SELECTOR, "input[name='price'][value='50-100']").click()
        driver.find_element(By.ID, "submitPref").click()
        time.sleep(1)
        
        # 确认弹层关闭
        is_closed = not pref_modal.is_displayed()
        if is_closed:
            results.append("[STEP 5] 问卷提交成功，弹层已关闭")
        else:
            results.append("[STEP 5] FAIL: 弹层未关闭")
            final_status = "FAIL"
        
        # Step 6: 打开location.html再返回
        driver.get("http://127.0.0.1:8080/assets/location.html")
        time.sleep(1)
        results.append("[STEP 6] 打开location.html")
        
        try:
            back_btn = driver.find_element(By.ID, "backToChat")
            back_btn.click()
            time.sleep(1)
            results.append("[STEP 6] 点击'返回聊天'")
        except:
            driver.get("http://127.0.0.1:8080")
            time.sleep(1)
            results.append("[STEP 6] 直接刷新首页")
        
        # Step 7: 验证偏好问卷不再弹出
        time.sleep(1)
        pref_modal_again = driver.find_element(By.ID, "prefModal")
        is_shown_again = pref_modal_again.is_displayed()
        
        if not is_shown_again:
            results.append("[STEP 7] 验证通过: 偏好问卷未再次弹出")
        else:
            results.append("[STEP 7] FAIL: 偏好问卷再次弹出")
            final_status = "FAIL"
        
    except Exception as e:
        results.append(f"[ERROR] {str(e)}")
        final_status = "FAIL"
    
    finally:
        driver.quit()
    
    print(f"\n{'='*60}")
    print(f"测试结果: {final_status}")
    print(f"{'='*60}")
    for r in results:
        print(r)
    print(f"{'='*60}\n")
    
    return final_status

if __name__ == "__main__":
    test_preference_flow()
