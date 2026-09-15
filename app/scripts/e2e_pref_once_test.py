#!/usr/bin/env python3
# -*- coding: utf-8 -*-
import time
from datetime import datetime
from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC
from selenium.webdriver.chrome.options import Options

def check_modal_visible(driver, modal_id):
    try:
        modal = driver.find_element(By.ID, modal_id)
        return driver.execute_script("""
            const modal = arguments[0];
            const style = window.getComputedStyle(modal);
            return style.display !== 'none' && style.visibility !== 'hidden';
        """, modal)
    except:
        return False

def main():
    timestamp = datetime.now().strftime("%Y%m%d%H%M%S")
    username = f"e2e_pref_once_{timestamp}"
    password = "123456"
    base_url = "http://127.0.0.1:8080"
    
    chrome_options = Options()
    chrome_options.add_argument('--headless')
    chrome_options.add_argument('--disable-gpu')
    chrome_options.add_argument('--no-sandbox')
    chrome_options.add_argument('--disable-dev-shm-usage')
    chrome_options.add_experimental_option('prefs', {
        'profile.default_content_setting_values.geolocation': 2
    })
    chrome_options.add_experimental_option('excludeSwitches', ['enable-logging'])
    
    driver = None
    result = {
        'status': 'FAIL',
        'first_modal_shown': False,
        'second_modal_shown': None,
        'evidence': []
    }
    
    try:
        driver = webdriver.Chrome(options=chrome_options)
        driver.set_page_load_timeout(30)
        wait = WebDriverWait(driver, 10)
        
        driver.get(base_url)
        time.sleep(1)
        result['evidence'].append(f"打开 {base_url}")
        
        register_btn = wait.until(EC.element_to_be_clickable((By.ID, "registerBtn")))
        register_btn.click()
        time.sleep(0.5)
        
        wait.until(EC.visibility_of_element_located((By.ID, "registerModal")))
        driver.find_element(By.ID, "regUsername").send_keys(username)
        driver.find_element(By.ID, "regPassword").send_keys(password)
        driver.find_element(By.ID, "registerSubmit").click()
        time.sleep(1)
        result['evidence'].append(f"注册用户 {username}")
        
        login_btn = wait.until(EC.element_to_be_clickable((By.ID, "loginBtn")))
        login_btn.click()
        time.sleep(0.5)
        
        wait.until(EC.visibility_of_element_located((By.ID, "loginModal")))
        driver.find_element(By.ID, "loginUsername").send_keys(username)
        driver.find_element(By.ID, "loginPassword").send_keys(password)
        driver.find_element(By.ID, "loginSubmit").click()
        time.sleep(2)
        result['evidence'].append("登录成功")
        
        if check_modal_visible(driver, "geoModal"):
            result['evidence'].append("定位授权弹层出现")
            deny_btn = driver.find_element(By.CSS_SELECTOR, "#geoModal button:last-child")
            deny_btn.click()
            time.sleep(1)
            result['evidence'].append("点击不允许定位")
        
        result['first_modal_shown'] = check_modal_visible(driver, "prefModal")
        if not result['first_modal_shown']:
            result['evidence'].append("ERROR: 偏好问卷未出现")
            print_result(result)
            return
        
        result['evidence'].append("偏好问卷弹层出现")
        
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
        result['evidence'].append("填写问卷")
        
        submit_btn = driver.find_element(By.ID, "prefSubmit")
        submit_btn.click()
        time.sleep(2)
        result['evidence'].append("提交问卷")
        
        driver.get(f"{base_url}/assets/location.html")
        time.sleep(1)
        result['evidence'].append("进入定位页")
        
        back_btn = wait.until(EC.element_to_be_clickable((By.CSS_SELECTOR, "button, a")))
        back_text = back_btn.text
        if "返回" in back_text or "back" in back_text.lower():
            back_btn.click()
            time.sleep(2)
            result['evidence'].append(f"点击'{back_text}'返回首页")
        else:
            driver.get(base_url)
            time.sleep(2)
            result['evidence'].append("直接导航回首页")
        
        result['second_modal_shown'] = check_modal_visible(driver, "prefModal")
        
        if result['first_modal_shown'] and not result['second_modal_shown']:
            result['status'] = 'PASS'
            result['evidence'].append("问卷未再次弹出")
        else:
            result['evidence'].append(f"ERROR: 问卷再次弹出={result['second_modal_shown']}")
        
        page_text = driver.execute_script("return document.body.innerText;")
        result['evidence'].append(f"页面文本片段: {page_text[:100]}")
        
    except Exception as e:
        result['evidence'].append(f"异常: {str(e)}")
    finally:
        if driver:
            driver.quit()
    
    print_result(result)

def print_result(result):
    print("\n========== 测试结果 ==========")
    print(f"结果: {result['status']}")
    print(f"问卷首次出现: {result['first_modal_shown']}")
    print(f"回首页后再次出现: {result['second_modal_shown']}")
    print("\n关键证据:")
    for ev in result['evidence']:
        print(f"  - {ev}")
    print("==============================")

if __name__ == "__main__":
    main()
