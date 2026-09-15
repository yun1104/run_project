import os
import time
import unittest

from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.support.ui import WebDriverWait


class TestUIFlow(unittest.TestCase):
    def setUp(self):
        options = webdriver.ChromeOptions()
        options.add_argument("--headless")
        options.add_argument("--disable-gpu")
        self.driver = webdriver.Chrome(options=options)
        self.wait = WebDriverWait(self.driver, 15)
        self.base_url = os.getenv("BASE_URL", "http://127.0.0.1:8080")

    def tearDown(self):
        self.driver.quit()

    def test_m1_e2e_031_032_033_ui_flow(self):
        driver = self.driver
        username = f"ui_test_{time.time_ns()}"

        driver.get(self.base_url)
        self.assertNotIn("hidden", driver.find_element(By.ID, "authModal").get_attribute("class"))

        driver.find_element(By.ID, "authRegisterBtn").click()
        driver.find_element(By.ID, "regUsername").send_keys(username)
        driver.find_element(By.ID, "regPassword").send_keys("123456")
        driver.find_element(By.ID, "regPassword2").send_keys("123456")
        driver.find_element(By.ID, "regSubmitBtn").click()
        self.wait.until(lambda d: "hidden" not in d.find_element(By.ID, "authModal").get_attribute("class"))

        auth_username = driver.find_element(By.ID, "authUsername")
        auth_username.clear()
        auth_username.send_keys(username)
        auth_password = driver.find_element(By.ID, "authPassword")
        auth_password.clear()
        auth_password.send_keys("123456")
        driver.find_element(By.ID, "authLoginBtn").click()
        self.wait.until(lambda d: "hidden" in d.find_element(By.ID, "authModal").get_attribute("class"))

        deny = driver.find_elements(By.ID, "loginLocDenyBtn")
        if deny and deny[0].is_displayed():
            deny[0].click()

        driver.find_element(By.ID, "promptInput").send_keys("预算30元，想吃辣")
        driver.find_element(By.ID, "sendBtn").click()
        self.wait.until(lambda d: len(d.find_elements(By.CSS_SELECTOR, ".cards .card")) > 0)

        driver.get(f"{self.base_url}/account")
        self.wait.until(lambda d: d.find_element(By.ID, "usernameText").text.strip() not in ("", "-"))

        driver.get(f"{self.base_url}/assets/location.html")
        self.assertNotIn("初始化失败", driver.find_element(By.TAG_NAME, "body").text)


if __name__ == "__main__":
    unittest.main()
