const puppeteer = require('puppeteer');

async function testPreferenceFlow() {
    const timestamp = new Date().toISOString().replace(/[-:T.]/g, '').slice(0, 14);
    const username = `e2e_pref_once_${timestamp}`;
    const password = '123456';
    
    const results = [];
    let finalStatus = 'PASS';
    
    const browser = await puppeteer.launch({ 
        headless: true,
        args: ['--no-sandbox', '--disable-setuid-sandbox']
    });
    
    try {
        const page = await browser.newPage();
        await page.setViewport({ width: 1280, height: 800 });
        
        // Step 1: 打开首页
        await page.goto('http://127.0.0.1:8080', { waitUntil: 'networkidle2' });
        const title = await page.title();
        results.push(`[STEP 1] 打开首页: ${title}`);
        
        // Step 2: 注册新用户
        await page.click('#showRegister');
        await page.waitForTimeout(500);
        
        await page.type('#regUsername', username);
        await page.type('#regPassword', password);
        await page.click('#registerModal button[type="submit"]');
        await page.waitForTimeout(1000);
        
        const alertText = await page.$eval('#alertMessage', el => el.textContent);
        results.push(`[STEP 2] 注册用户 ${username}: ${alertText}`);
        
        // 登录
        await page.type('#username', username);
        await page.type('#password', password);
        await page.click('form button[type="submit"]');
        await page.waitForTimeout(2000);
        
        // Step 3: 处理定位授权弹层
        try {
            const denyBtn = await page.$('#denyLocation');
            if (denyBtn) {
                const isVisible = await page.evaluate(el => el.offsetParent !== null, denyBtn);
                if (isVisible) {
                    await denyBtn.click();
                    await page.waitForTimeout(500);
                    results.push('[STEP 3] 点击定位授权"不允许"');
                }
            } else {
                results.push('[STEP 3] 未出现定位授权弹层');
            }
        } catch (e) {
            results.push('[STEP 3] 未出现定位授权弹层');
        }
        
        // Step 4: 验证偏好问卷弹层出现
        await page.waitForTimeout(1000);
        const prefModal = await page.$('#prefModal');
        const isVisible = await page.evaluate(el => {
            const style = window.getComputedStyle(el);
            return style.display !== 'none' && el.offsetParent !== null;
        }, prefModal);
        
        if (isVisible) {
            const modalTitle = await page.$eval('#prefModal .modal-title', el => el.textContent);
            results.push(`[STEP 4] 偏好问卷弹层出现: '${modalTitle}'`);
        } else {
            results.push('[STEP 4] FAIL: 偏好问卷弹层未出现');
            finalStatus = 'FAIL';
            return;
        }
        
        // Step 5: 完成问卷并提交
        await page.click('input[name="cuisine"][value="川菜"]');
        await page.click('input[name="price"][value="50-100"]');
        await page.click('#submitPref');
        await page.waitForTimeout(1000);
        
        // 确认弹层关闭
        const isClosed = await page.evaluate(el => {
            const style = window.getComputedStyle(el);
            return style.display === 'none' || el.offsetParent === null;
        }, prefModal);
        
        if (isClosed) {
            results.push('[STEP 5] 问卷提交成功，弹层已关闭');
        } else {
            results.push('[STEP 5] FAIL: 弹层未关闭');
            finalStatus = 'FAIL';
        }
        
        // Step 6: 打开location.html再返回
        await page.goto('http://127.0.0.1:8080/assets/location.html', { waitUntil: 'networkidle2' });
        await page.waitForTimeout(1000);
        results.push('[STEP 6] 打开location.html');
        
        try {
            const backBtn = await page.$('#backToChat');
            if (backBtn) {
                await backBtn.click();
                await page.waitForTimeout(1000);
                results.push('[STEP 6] 点击"返回聊天"');
            } else {
                await page.goto('http://127.0.0.1:8080', { waitUntil: 'networkidle2' });
                await page.waitForTimeout(1000);
                results.push('[STEP 6] 直接刷新首页');
            }
        } catch (e) {
            await page.goto('http://127.0.0.1:8080', { waitUntil: 'networkidle2' });
            await page.waitForTimeout(1000);
            results.push('[STEP 6] 直接刷新首页');
        }
        
        // Step 7: 验证偏好问卷不再弹出
        await page.waitForTimeout(1000);
        const prefModalAgain = await page.$('#prefModal');
        const isShownAgain = await page.evaluate(el => {
            const style = window.getComputedStyle(el);
            return style.display !== 'none' && el.offsetParent !== null;
        }, prefModalAgain);
        
        if (!isShownAgain) {
            results.push('[STEP 7] 验证通过: 偏好问卷未再次弹出');
        } else {
            results.push('[STEP 7] FAIL: 偏好问卷再次弹出');
            finalStatus = 'FAIL';
        }
        
    } catch (error) {
        results.push(`[ERROR] ${error.message}`);
        finalStatus = 'FAIL';
    } finally {
        await browser.close();
    }
    
    console.log('\n' + '='.repeat(60));
    console.log(`测试结果: ${finalStatus}`);
    console.log('='.repeat(60));
    results.forEach(r => console.log(r));
    console.log('='.repeat(60) + '\n');
    
    return finalStatus;
}

testPreferenceFlow().catch(console.error);
