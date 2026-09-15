param(
  [string]$BaseUrl = "http://127.0.0.1:8080",
  [int]$TimeoutSec = 10
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
    [hashtable]$Headers = @{}
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
    foreach ($name in $Headers.Keys) {
      [void]$request.Headers.TryAddWithoutValidation($name, [string]$Headers[$name])
    }

    if ($null -ne $Body) {
      $json = $Body | ConvertTo-Json -Depth 10 -Compress
      $request.Content = New-Object System.Net.Http.StringContent($json, [System.Text.Encoding]::UTF8, "application/json")
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

function Get-BodyCode {
  param([object]$Response)

  if ($null -eq $Response -or $null -eq $Response.Body -or $null -eq $Response.Body.code) {
    return $null
  }
  return [int]$Response.Body.code
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

function Test-GatewayReady {
  try {
    $response = Invoke-WebRequest -Uri "$BaseUrl/" -UseBasicParsing -TimeoutSec $TimeoutSec
    return ($response.StatusCode -ge 200 -and $response.StatusCode -lt 500)
  } catch {
    return $false
  }
}

if (-not (Test-GatewayReady)) {
  throw "Gateway is not reachable at $BaseUrl. Start it first, for example: .\scripts\run.ps1"
}

$runId = [DateTimeOffset]::Now.ToUnixTimeMilliseconds()
$username = "token_edge_$runId"
$password = "123456"

Write-Host "Running TokenEdgeTest against $BaseUrl"

$registerResp = Invoke-JsonApi -Method POST -Path "/api/v1/user/register" -Body @{
  username = $username
  password = $password
}
if ($registerResp.StatusCode -ne 200 -or (Get-BodyCode $registerResp) -ne 0) {
  throw "TokenEdgeTest setup failed: seed user could not be registered. Raw: $($registerResp.Raw)"
}

$loginResp = Invoke-JsonApi -Method POST -Path "/api/v1/user/login" -Body @{
  username = $username
  password = $password
}
if ($loginResp.StatusCode -ne 200 -or (Get-BodyCode $loginResp) -ne 0 -or [string]::IsNullOrWhiteSpace([string]$loginResp.Body.token)) {
  throw "TokenEdgeTest setup failed: seed user could not log in. Raw: $($loginResp.Raw)"
}
$token = [string]$loginResp.Body.token

$resp = Invoke-JsonApi -Method GET -Path "/api/v1/user/me"
$ok = ($resp.StatusCode -eq 401 -and (Get-BodyCode $resp) -eq 401)
Assert-Condition -Id "M1-EXT-017" -Title "missing Authorization header" -Ok $ok -Failure "expected HTTP 401 and code=401, got HTTP $($resp.StatusCode), code=$(Get-BodyCode $resp)" -Response $resp

$resp = Invoke-JsonApi -Method GET -Path "/api/v1/user/me" -Headers @{
  Authorization = "Token $token"
}
$ok = ($resp.StatusCode -eq 401 -and (Get-BodyCode $resp) -eq 401)
Assert-Condition -Id "M1-EXT-018" -Title "non-Bearer Authorization scheme" -Ok $ok -Failure "expected HTTP 401 and code=401, got HTTP $($resp.StatusCode), code=$(Get-BodyCode $resp)" -Response $resp

$resp = Invoke-JsonApi -Method GET -Path "/api/v1/user/me" -Headers @{
  Authorization = "bearer $token"
}
$ok = ($resp.StatusCode -eq 200 -and (Get-BodyCode $resp) -eq 0 -and $resp.Body.data.username -eq $username)
Assert-Condition -Id "M1-EXT-019" -Title "lowercase bearer scheme" -Ok $ok -Failure "expected HTTP 200 and code=0, got HTTP $($resp.StatusCode), code=$(Get-BodyCode $resp)" -Response $resp

$resp = Invoke-JsonApi -Method GET -Path "/api/v1/user/me" -Headers @{
  Authorization = "Bearer u1-999999999999"
}
$ok = ($resp.StatusCode -eq 401 -and (Get-BodyCode $resp) -eq 401)
Assert-Condition -Id "M1-EXT-020" -Title "forged token with similar format" -Ok $ok -Failure "expected HTTP 401 and code=401, got HTTP $($resp.StatusCode), code=$(Get-BodyCode $resp)" -Response $resp

$resp = Invoke-JsonApi -Method GET -Path "/api/v1/user/me" -Headers @{
  Authorization = "Bearer $token`nX-Test: 1"
}
$code = Get-BodyCode $resp
$ok = (($resp.StatusCode -eq 401 -and $code -eq 401) -or ($resp.StatusCode -eq 400) -or ($resp.StatusCode -eq 0 -and $resp.Error))
Assert-Condition -Id "M1-EXT-021" -Title "token with newline/control characters" -Ok $ok -Failure "expected request rejection or HTTP 400/401, got HTTP $($resp.StatusCode), code=$code" -Response $resp

Write-Host ""
Write-Host "TokenEdgeTest summary: Total=$script:Total Passed=$script:Passed Failed=$script:Failed"

if ($script:Failed -gt 0) {
  Write-Host ""
  Write-Host "Failures:"
  $script:Failures | ForEach-Object { Write-Host $_ -ForegroundColor Red }
  exit 1
}

exit 0
