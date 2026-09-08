# 单元测试自动化说明

本文档说明如何一键运行本项目的自动化单元测试。

## 测试范围

当前测试脚本覆盖以下模块：

- API网关中的密码哈希、推荐规则、鉴权校验、Host限制中间件
- 用户服务中的密码哈希、偏好数组序列化和反序列化
- 偏好分析服务中的订单偏好分析、偏好合并、去重辅助函数
- AI客户端的请求构造、响应解析、推荐分数解析
- 公共并发组件中的LRU缓存、WorkerPool、SafeCounter、SafeMap
- 公共中间件中的限流器和熔断器

根据当前项目实际功能，测试范围已排除订单支付、订单查询及并发下单相关用例。

## 一键运行

在项目根目录执行：

```bat
run_tests.bat
```

或直接执行：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\run_unit_tests.ps1
```

脚本会自动查找 Go：

1. 优先使用 `C:\Go\bin\go.exe`
2. 其次使用项目内 `.tools\go\bin\go.exe`
3. 如果都不存在，会尝试下载 Go 1.22.6 便携版到 `.tools`

## 常用参数

显示详细测试日志：

```bat
run_tests.bat -Verbose
```

启用数据竞争检测：

```bat
run_tests.bat -Race
```

同时启用详细日志和竞争检测：

```bat
run_tests.bat -Verbose -Race
```

## 预期结果

测试全部通过时，终端会输出：

```text
全部单元测试通过。
```

如果某个用例失败，Go测试框架会显示失败的包名、测试函数名和断言信息。

## 注意事项

- 这些测试是单元测试，不依赖真实MySQL、Redis、Kafka或外部AI服务。
- AI客户端测试使用本地 `httptest` 模拟HTTP服务，不会访问真实大模型接口。
- 如果首次运行需要下载Go，必须保证网络可访问 `https://go.dev/dl/`。
