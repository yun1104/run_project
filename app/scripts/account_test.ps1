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
    return [pscustomobject]@{
      StatusCode = [int]$response.StatusCode
      Body = Convert-ResponseJson -Content $content
      Raw = $content
      Error = ""
    }
  } catch {
    return [pscustomobject]@{
      StatusCode = 0
      Body = $null
      Raw = ""
      Error = $_.Exception.Message
    }
  } finally {
    if ($null -ne $request) {
      $request.Dispose()
    }
    $client.Dispose()
  }
}

function Assert-AccountResponse {
  param(
    [string]$Id,
    [string]$Title,
    [object]$Response,
    [int]$ExpectedHttpStatus,
    [int]$ExpectedCode,
    [int64]$ExpectedUserId,
    [string]$ExpectedUsername
  )

  $script:Total++
  $errors = @()

  if ($Response.StatusCode -ne $ExpectedHttpStatus) {
    $errors += "HTTP expected $ExpectedHttpStatus, got $($Response.StatusCode)"
  }

  if ($null -eq $Response.Body) {
    $errors += "response body is not valid JSON"
  } elseif ([int]$Response.Body.code -ne $ExpectedCode) {
    $errors += "body.code expected $ExpectedCode, got $($Response.Body.code)"
  }

  if ($null -eq $Response.Body -or $null -eq $Response.Body.data) {
    $errors += "data is missing"
  } else {
    if ([int64]$Response.Body.data.user_id -ne $ExpectedUserId) {
      $errors += "data.user_id expected $ExpectedUserId, got $($Response.Body.data.user_id)"
    }
    if ($Response.Body.data.username -ne $ExpectedUsername) {
      $errors += "data.username expected $ExpectedUsername, got $($Response.Body.data.username)"
    }
  }

  if ($errors.Count -eq 0) {
    $script:Passed++
    Write-Host "[PASS] $Id $Title"
    return
  }

  $script:Failed++
  $message = "[FAIL] $Id $Title - $($errors -join '; ')"
  $script:Failures += $message
  Write-Host $message -ForegroundColor Red
  if ($Response.Raw) {
    Write-Host "       Raw: $($Response.Raw)"
  } elseif ($Response.Error) {
    Write-Host "       Error: $($Response.Error)"
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
$username = "account_ok_$runId"
$password = "123456"

Write-Host "Running AccountTest against $BaseUrl"

$registerResp = Invoke-JsonApi -Method POST -Path "/api/v1/user/register" -Body @{
  username = $username
  password = $password
}
if ($registerResp.StatusCode -ne 200 -or $null -eq $registerResp.Body -or [int]$registerResp.Body.code -ne 0 -or -not $registerResp.Body.data.user_id) {
  throw "AccountTest setup failed: seed user could not be registered. Raw: $($registerResp.Raw)"
}
$userId = [int64]$registerResp.Body.data.user_id

$loginResp = Invoke-JsonApi -Method POST -Path "/api/v1/user/login" -Body @{
  username = $username
  password = $password
}
if ($loginResp.StatusCode -ne 200 -or $null -eq $loginResp.Body -or [int]$loginResp.Body.code -ne 0 -or [string]::IsNullOrWhiteSpace([string]$loginResp.Body.token)) {
  throw "AccountTest setup failed: seed user could not log in. Raw: $($loginResp.Raw)"
}
$token = [string]$loginResp.Body.token

$resp = Invoke-JsonApi -Method GET -Path "/api/v1/user/me" -Headers @{
  Authorization = "Bearer $token"
}
Assert-AccountResponse -Id "M1-UT-017" -Title "query registered user" -Response $resp -ExpectedHttpStatus 200 -ExpectedCode 0 -ExpectedUserId $userId -ExpectedUsername $username

Write-Host ""
Write-Host "AccountTest summary: Total=$script:Total Passed=$script:Passed Failed=$script:Failed"

if ($script:Failed -gt 0) {
  Write-Host ""
  Write-Host "Failures:"
  $script:Failures | ForEach-Object { Write-Host $_ -ForegroundColor Red }
  exit 1
}

exit 0
