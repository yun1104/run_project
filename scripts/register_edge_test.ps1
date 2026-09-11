param(
  [string]$BaseUrl = "http://127.0.0.1:8080",
  [int]$TimeoutSec = 10,
  [int]$BatchCount = 100,
  [int]$ConcurrentCount = 20
)

$ErrorActionPreference = "Stop"

Add-Type -AssemblyName System.Net.Http

$script:Total = 0
$script:Passed = 0
$script:Failed = 0
$script:Failures = @()

function Convert-ResponseJson {
  param([string]$Content)

  if ([string]::IsNullOrWhiteSpace($Content)) {
    return $null
  }

  try {
    return $Content | ConvertFrom-Json
  } catch {
    return $null
  }
}

function New-ApiResult {
  param(
    [int]$StatusCode,
    [string]$Content,
    [string]$Error = ""
  )

  return [pscustomobject]@{
    StatusCode = $StatusCode
    Body = Convert-ResponseJson -Content $Content
    Raw = $Content
    Error = $Error
  }
}

function Invoke-JsonApi {
  param(
    [ValidateSet("GET", "POST", "PUT", "DELETE")]
    [string]$Method,
    [string]$Path,
    [object]$Body = $null,
    [string]$RawBody = $null,
    [string]$ContentType = "application/json"
  )

  $uri = "$BaseUrl$Path"
  $client = New-Object System.Net.Http.HttpClient
  $client.Timeout = [TimeSpan]::FromSeconds($TimeoutSec)

  switch ($Method) {
    "GET" { $httpMethod = [System.Net.Http.HttpMethod]::Get }
    "POST" { $httpMethod = [System.Net.Http.HttpMethod]::Post }
    "PUT" { $httpMethod = [System.Net.Http.HttpMethod]::Put }
    "DELETE" { $httpMethod = [System.Net.Http.HttpMethod]::Delete }
    default { throw "Unsupported HTTP method: $Method" }
  }

  $request = New-Object System.Net.Http.HttpRequestMessage($httpMethod, $uri)
  try {
    if ($PSBoundParameters.ContainsKey("RawBody")) {
      $request.Content = New-Object System.Net.Http.StringContent($RawBody, [System.Text.Encoding]::UTF8, $ContentType)
    } elseif ($null -ne $Body) {
      $json = $Body | ConvertTo-Json -Depth 10 -Compress
      $request.Content = New-Object System.Net.Http.StringContent($json, [System.Text.Encoding]::UTF8, $ContentType)
    }

    $response = $client.SendAsync($request).GetAwaiter().GetResult()
    $content = $response.Content.ReadAsStringAsync().GetAwaiter().GetResult()
    return New-ApiResult -StatusCode ([int]$response.StatusCode) -Content $content
  } catch {
    return New-ApiResult -StatusCode 0 -Content "" -Error $_.Exception.Message
  } finally {
    if ($null -ne $request) {
      $request.Dispose()
    }
    $client.Dispose()
  }
}

function Assert-Condition {
  param(
    [string]$Id,
    [string]$Title,
    [bool]$Ok,
    [string]$Failure,
    [object]$Response = $null
  )

  $script:Total++
  if ($Ok) {
    $script:Passed++
    Write-Host "[PASS] $Id $Title"
    return
  }

  $script:Failed++
  $message = "[FAIL] $Id $Title - $Failure"
  $script:Failures += $message
  Write-Host $message -ForegroundColor Red
  if ($null -ne $Response) {
    if ($Response.Raw) {
      Write-Host "       Raw: $($Response.Raw)"
    } elseif ($Response.Error) {
      Write-Host "       Error: $($Response.Error)"
    }
  }
}

function Get-BodyCode {
  param([object]$Response)

  if ($null -eq $Response -or $null -eq $Response.Body -or $null -eq $Response.Body.code) {
    return $null
  }
  return [int]$Response.Body.code
}

function Test-GatewayReady {
  try {
    $response = Invoke-WebRequest -Uri "$BaseUrl/" -UseBasicParsing -TimeoutSec $TimeoutSec
    return ($response.StatusCode -ge 200 -and $response.StatusCode -lt 500)
  } catch {
    return $false
  }
}

function Invoke-ConcurrentRegister {
  param(
    [string]$Username,
    [string]$Password,
    [int]$Count
  )

  $client = New-Object System.Net.Http.HttpClient
  $client.Timeout = [TimeSpan]::FromSeconds($TimeoutSec)
  $tasks = New-Object System.Collections.Generic.List[object]
  $uri = "$BaseUrl/api/v1/user/register"
  $json = (@{ username = $Username; password = $Password } | ConvertTo-Json -Compress)

  try {
    for ($i = 0; $i -lt $Count; $i++) {
      $content = New-Object System.Net.Http.StringContent($json, [System.Text.Encoding]::UTF8, "application/json")
      $tasks.Add($client.PostAsync($uri, $content))
    }

    [System.Threading.Tasks.Task]::WaitAll($tasks.ToArray())

    $results = @()
    foreach ($task in $tasks) {
      $response = $task.Result
      $content = $response.Content.ReadAsStringAsync().GetAwaiter().GetResult()
      $results += New-ApiResult -StatusCode ([int]$response.StatusCode) -Content $content
      $response.Dispose()
    }
    return $results
  } finally {
    $client.Dispose()
  }
}

if (-not (Test-GatewayReady)) {
  throw "Gateway is not reachable at $BaseUrl. Start it first, for example: .\scripts\run.ps1"
}

$runId = [DateTimeOffset]::Now.ToUnixTimeMilliseconds()

Write-Host "Running RegisterEdgeTest against $BaseUrl"

$longUsername = "u" * 80
$resp = Invoke-JsonApi -Method POST -Path "/api/v1/user/register" -Body @{
  username = "edge_${runId}_$longUsername"
  password = "123456"
}
$ok = ($resp.StatusCode -eq 400 -and (Get-BodyCode $resp) -eq 400)
Assert-Condition -Id "M1-EXT-001" -Title "too long username" -Ok $ok -Failure "expected HTTP 400 and code=400, got HTTP $($resp.StatusCode), code=$(Get-BodyCode $resp)" -Response $resp

$longPassword = "p" * 1200
$resp = Invoke-JsonApi -Method POST -Path "/api/v1/user/register" -Body @{
  username = "edge_longpwd_$runId"
  password = $longPassword
}
$code = Get-BodyCode $resp
$ok = ($resp.StatusCode -in @(200, 400) -and $code -in @(0, 400))
Assert-Condition -Id "M1-EXT-002" -Title "too long password" -Ok $ok -Failure "expected controlled HTTP 200/400 without 5xx, got HTTP $($resp.StatusCode), code=$code" -Response $resp

$resp = Invoke-JsonApi -Method POST -Path "/api/v1/user/register" -Body @{
  username = "测试用户_$runId"
  password = "123456"
}
$code = Get-BodyCode $resp
$ok = ($resp.StatusCode -in @(200, 400) -and $code -in @(0, 400))
Assert-Condition -Id "M1-EXT-003" -Title "Chinese username" -Ok $ok -Failure "expected controlled HTTP 200/400 without garbled error, got HTTP $($resp.StatusCode), code=$code" -Response $resp

$resp = Invoke-JsonApi -Method POST -Path "/api/v1/user/register" -Body @{
  username = "edge!@#_$runId"
  password = "123456"
}
$code = Get-BodyCode $resp
$ok = ($resp.StatusCode -in @(200, 400) -and $code -in @(0, 400))
Assert-Condition -Id "M1-EXT-004" -Title "special character username" -Ok $ok -Failure "expected controlled HTTP 200/400 without 5xx, got HTTP $($resp.StatusCode), code=$code" -Response $resp

$resp = Invoke-JsonApi -Method POST -Path "/api/v1/user/register" -Body @{
  username = "' OR 1=1 --_$runId"
  password = "123456"
}
$code = Get-BodyCode $resp
$ok = ($resp.StatusCode -in @(200, 400) -and $code -in @(0, 400))
Assert-Condition -Id "M1-EXT-005" -Title "SQL injection shaped username" -Ok $ok -Failure "expected controlled HTTP 200/400 and no bypass/5xx, got HTTP $($resp.StatusCode), code=$code" -Response $resp

$resp = Invoke-JsonApi -Method POST -Path "/api/v1/user/register" -Body @{
  password = "123456"
}
$ok = ($resp.StatusCode -eq 400 -and (Get-BodyCode $resp) -eq 400)
Assert-Condition -Id "M1-EXT-006" -Title "missing username field" -Ok $ok -Failure "expected HTTP 400 and code=400, got HTTP $($resp.StatusCode), code=$(Get-BodyCode $resp)" -Response $resp

$resp = Invoke-JsonApi -Method POST -Path "/api/v1/user/register" -RawBody '{"username":12345,"password":"123456"}'
$ok = ($resp.StatusCode -eq 400 -and (Get-BodyCode $resp) -eq 400)
Assert-Condition -Id "M1-EXT-007" -Title "username type mismatch" -Ok $ok -Failure "expected HTTP 400 and code=400, got HTTP $($resp.StatusCode), code=$(Get-BodyCode $resp)" -Response $resp

$resp = Invoke-JsonApi -Method POST -Path "/api/v1/user/register" -RawBody '{"username":"abc",'
$ok = ($resp.StatusCode -eq 400 -and (Get-BodyCode $resp) -eq 400)
Assert-Condition -Id "M1-EXT-008" -Title "invalid JSON body" -Ok $ok -Failure "expected HTTP 400 and code=400, got HTTP $($resp.StatusCode), code=$(Get-BodyCode $resp)" -Response $resp

$resp = Invoke-JsonApi -Method POST -Path "/api/v1/user/register" -RawBody "username=edge_plain_$runId&password=123456" -ContentType "text/plain"
$ok = ($resp.StatusCode -eq 400 -and (Get-BodyCode $resp) -eq 400)
Assert-Condition -Id "M1-EXT-009" -Title "wrong content type" -Ok $ok -Failure "expected HTTP 400 and code=400, got HTTP $($resp.StatusCode), code=$(Get-BodyCode $resp)" -Response $resp

$concurrentUser = "edge_concurrent_$runId"
$responses = Invoke-ConcurrentRegister -Username $concurrentUser -Password "123456" -Count $ConcurrentCount
$successCount = @($responses | Where-Object { $_.StatusCode -eq 200 -and (Get-BodyCode $_) -eq 0 }).Count
$conflictCount = @($responses | Where-Object { $_.StatusCode -eq 409 -and (Get-BodyCode $_) -eq 409 }).Count
$badCount = @($responses | Where-Object { $_.StatusCode -ge 500 -or $_.StatusCode -eq 0 }).Count
$ok = ($successCount -eq 1 -and $conflictCount -eq ($ConcurrentCount - 1) -and $badCount -eq 0)
Assert-Condition -Id "M1-EXT-010" -Title "concurrent duplicate username registration" -Ok $ok -Failure "expected 1 success, $($ConcurrentCount - 1) conflicts, 0 errors; got success=$successCount conflict=$conflictCount bad=$badCount"

$batchFailed = 0
$batchPassed = 0
$batchSamples = @()
$sw = [System.Diagnostics.Stopwatch]::StartNew()
for ($i = 0; $i -lt $BatchCount; $i++) {
  $batchUser = "edge_batch_${runId}_$i"
  $resp = Invoke-JsonApi -Method POST -Path "/api/v1/user/register" -Body @{
    username = $batchUser
    password = "123456"
  }
  $respCode = Get-BodyCode $resp
  if ($resp.StatusCode -eq 200 -and $respCode -eq 0) {
    $batchPassed++
  } else {
    $batchFailed++
    if ($batchSamples.Count -lt 3) {
      $batchSamples += "user=$batchUser HTTP=$($resp.StatusCode) code=$respCode raw=$($resp.Raw) error=$($resp.Error)"
    }
  }
}
$sw.Stop()
$ok = ($batchFailed -eq 0)
Assert-Condition -Id "M1-EXT-011" -Title "batch register different usernames" -Ok $ok -Failure "expected all $BatchCount registrations to succeed, passed=$batchPassed failed=$batchFailed elapsedMs=$($sw.ElapsedMilliseconds); samples=$($batchSamples -join ' || ')"

Write-Host ""
Write-Host "RegisterEdgeTest summary: Total=$script:Total Passed=$script:Passed Failed=$script:Failed"

if ($script:Failed -gt 0) {
  Write-Host ""
  Write-Host "Failures:"
  $script:Failures | ForEach-Object { Write-Host $_ -ForegroundColor Red }
  exit 1
}

exit 0
