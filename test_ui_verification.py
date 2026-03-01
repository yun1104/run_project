import time
from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC
from selenium.webdriver.common.keys import Keys

def test_ui():
    driver = webdriver.Chrome()
    
    try:
        # 1) 打开并强制刷新
        print("步骤1: 打开 http://127.0.0.1:8080/")
        driver.get("http://127.0.0.1:8080/")
        time.sleep(1)
        
        # 清除localStorage以确保是全新登录
        driver.execute_script("localStorage.clear();")
        
        driver.refresh()
        time.sleep(2)
        
        # 2) 登录
        print("步骤2: 登录 llm_two_call_user3 / 123456")
        username_input = WebDriverWait(driver, 10).until(
            EC.presence_of_element_located((By.ID, "authUsername"))
        )
        password_input = driver.find_element(By.ID, "authPassword")
        login_btn = driver.find_element(By.ID, "authLoginBtn")
        
        username_input.send_keys("llm_two_call_user3")
        password_input.send_keys("123456")
        login_btn.click()
        
        time.sleep(3)
        
        # 跳过偏好设置
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
        
        time.sleep(1)
        
        # 刷新页面以触发"欢迎回来"消息
        print("刷新页面以查看'欢迎回来'消息")
        driver.refresh()
        time.sleep(2)
        
        # 3) 检查欢迎消息
        print("步骤3: 检查欢迎消息")
        all_messages = WebDriverWait(driver, 10).until(
            EC.presence_of_all_elements_located((By.CSS_SELECTOR, ".message.assistant"))
        )
        
        # 找到包含"欢迎回来"的消息
        welcome_bubble = None
        for msg in all_messages:
            if "欢迎回来" in msg.text:
                welcome_bubble = msg
                break
        
        if not welcome_bubble and all_messages:
            # 如果没找到"欢迎回来"，使用最后一条助手消息
            welcome_bubble = all_messages[-1]
        
        messages = [welcome_bubble] if welcome_bubble else []
        
        welcome_single_line = False
        welcome_text = ""
        
        if messages:
            first_msg = messages[0]
            welcome_text = first_msg.text
            # 检查是否单行：检查文本中是否有换行符
            welcome_single_line = '\n' not in welcome_text
            print(f"  欢迎消息: {repr(welcome_text)}")
            print(f"  是否单行: {welcome_single_line}")
            if not welcome_single_line:
                print(f"  问题：文本包含换行符")
        
        # 4) 发送"你好"
        print("步骤4: 发送'你好'")
        user_input = driver.find_element(By.ID, "promptInput")
        send_btn = driver.find_element(By.ID, "sendBtn")
        
        user_input.send_keys("你好")
        send_btn.click()
        
        time.sleep(1)
        
        # 检查用户气泡
        user_bubbles = driver.find_elements(By.CSS_SELECTOR, ".message.user")
        user_bubble_single_line = False
        user_bubble_text = ""
        
        if user_bubbles:
            last_user_msg = user_bubbles[-1]
            user_bubble_text = last_user_msg.text
            # 检查是否单行：检查文本中是否有换行符
            user_bubble_single_line = '\n' not in user_bubble_text
            print(f"  用户消息: {repr(user_bubble_text)}")
            print(f"  是否单行: {user_bubble_single_line}")
            if not user_bubble_single_line:
                print(f"  问题：文本包含换行符")
        
        # 5) 返回结果
        print("\n=== 验证结果 ===")
        if welcome_single_line and user_bubble_single_line:
            print("✓ 通过")
            print(f"  - 欢迎消息单行: 是 ('{welcome_text}')")
            print(f"  - 用户气泡单行: 是 ('{user_bubble_text}')")
        else:
            print("✗ 失败")
            print(f"  - 欢迎消息单行: {'是' if welcome_single_line else '否'} ('{welcome_text}')")
            print(f"  - 用户气泡单行: {'是' if user_bubble_single_line else '否'} ('{user_bubble_text}')")
        
        time.sleep(2)
        
    except Exception as e:
        print(f"错误: {e}")
        import traceback
        traceback.print_exc()
    
    finally:
        driver.quit()

if __name__ == "__main__":
    test_ui()
