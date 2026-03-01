# -*- coding: utf-8 -*-
from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC
from selenium.webdriver.common.keys import Keys
import time

driver = webdriver.Chrome()
try:
    driver.get("http://127.0.0.1:8080/")
    time.sleep(1)
    
    driver.refresh()
    time.sleep(2)
    
    username_input = WebDriverWait(driver, 10).until(
        EC.presence_of_element_located((By.ID, "authUsername"))
    )
    password_input = driver.find_element(By.ID, "authPassword")
    login_button = driver.find_element(By.ID, "authLoginBtn")
    
    username_input.send_keys("llm_two_call_user3")
    password_input.send_keys("123456")
    login_button.click()
    
    time.sleep(2)
    
    try:
        pref_modal = driver.find_element(By.ID, "prefModal")
        if "hidden" not in pref_modal.get_attribute("class"):
            for _ in range(10):
                try:
                    options = driver.find_elements(By.CSS_SELECTOR, "#prefOptions button")
                    if options:
                        options[0].click()
                        time.sleep(0.3)
                    next_btn = driver.find_element(By.ID, "prefNextBtn")
                    next_btn.click()
                    time.sleep(0.5)
                except:
                    break
    except:
        pass
    
    welcome_bubble = WebDriverWait(driver, 10).until(
        EC.presence_of_element_located((By.CSS_SELECTOR, ".message.assistant"))
    )
    welcome_text = welcome_bubble.text
    print(f"欢迎气泡文本: {repr(welcome_text)}")
    
    input_box = driver.find_element(By.ID, "promptInput")
    input_box.send_keys("你好")
    send_button = driver.find_element(By.ID, "sendBtn")
    send_button.click()
    
    time.sleep(1)
    
    user_bubbles = driver.find_elements(By.CSS_SELECTOR, ".message.user")
    if user_bubbles:
        last_user_text = user_bubbles[-1].text
        print(f"用户气泡文本: {repr(last_user_text)}")
    
    has_welcome_issue = "欢迎回来，\n123456" in welcome_text or "欢迎回来，\\n123456" in repr(welcome_text)
    has_user_issue = "你\n好" in last_user_text or len(last_user_text.split('\n')) > 1
    
    if has_welcome_issue or has_user_issue:
        print("\nFAIL")
    else:
        print("\nPASS")
    
    time.sleep(3)
    
except Exception as e:
    print(f"错误: {e}")
    try:
        username_input = driver.find_element(By.ID, "authUsername")
        password_input = driver.find_element(By.ID, "authPassword")
        login_button = driver.find_element(By.ID, "authLoginBtn")
        
        username_input.clear()
        password_input.clear()
        username_input.send_keys("chat_guard_user3")
        password_input.send_keys("123456")
        login_button.click()
        
        time.sleep(2)
        
        welcome_bubble = WebDriverWait(driver, 10).until(
            EC.presence_of_element_located((By.CSS_SELECTOR, ".message.assistant"))
        )
        welcome_text = welcome_bubble.text
        print(f"欢迎气泡文本: {repr(welcome_text)}")
        
        input_box = driver.find_element(By.ID, "promptInput")
        input_box.send_keys("你好")
        send_button = driver.find_element(By.ID, "sendBtn")
        send_button.click()
        
        time.sleep(1)
        
        user_bubbles = driver.find_elements(By.CSS_SELECTOR, ".message.user")
        if user_bubbles:
            last_user_text = user_bubbles[-1].text
            print(f"用户气泡文本: {repr(last_user_text)}")
        
        has_welcome_issue = "欢迎回来，\n123456" in welcome_text
        has_user_issue = "你\n好" in last_user_text
        
        if has_welcome_issue or has_user_issue:
            print("\nFAIL")
        else:
            print("\nPASS")
            
    except Exception as e2:
        print(f"备用登录也失败: {e2}")
finally:
    driver.quit()
