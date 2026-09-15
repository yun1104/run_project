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
    [object]$Body = $null
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

function Assert-LoginResponse {
  param(
    [string]$Id,
    [string]$Title,
    [object]$Response,
    [int]$ExpectedHttpStatus,
    [int]$ExpectedCode,
    [switch]$RequireToken,
    [string]$ExpectedUsername = ""
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

  if ($RequireToken) {
    if ($null -eq $Response.Body -or [string]::IsNullOrWhiteSpace([string]$Response.Body.token)) {
      $errors += "token is missing"
    }
    if ($null -eq $Response.Body -or -not $Response.Body.user_id) {
      $errors += "user_id is missing"
    }
    if ($ExpectedUsername -ne "" -and ($null -eq $Response.Body -or $Response.Body.username -ne $ExpectedUsername)) {
      $errors += "username expected $ExpectedUsername, got $($Response.Body.username)"
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
$username = "login_ok_$runId"
$password = "123456"

Write-Host "Running LoginTest against $BaseUrl"

$setupResp = Invoke-JsonApi -Method POST -Path "/api/v1/user/register" -Body @{
  username = $username
  password = $password
}
if ($setupResp.StatusCode -ne 200 -or $null -eq $setupResp.Body -or [int]$setupResp.Body.code -ne 0) {
  throw "LoginTest setup failed: seed user could not be registered. Raw: $($setupResp.Raw)"
}

$resp = Invoke-JsonApi -Method POST -Path "/api/v1/user/login" -Body @{
  username = $username
  password = $password
}
Assert-LoginResponse -Id "M1-UT-008" -Title "correct credentials" -Response $resp -ExpectedHttpStatus 200 -ExpectedCode 0 -RequireToken -ExpectedUsername $username

$resp = Invoke-JsonApi -Method POST -Path "/api/v1/user/login" -Body @{
  username = "  $username  "
  password = $password
}
Assert-LoginResponse -Id "M1-UT-009" -Title "username with leading and trailing spaces" -Response $resp -ExpectedHttpStatus 200 -ExpectedCode 0 -RequireToken -ExpectedUsername $username

$resp = Invoke-JsonApi -Method POST -Path "/api/v1/user/login" -Body @{
  username = $username
  password = "654321"
}
Assert-LoginResponse -Id "M1-UT-010" -Title "wrong password" -Response $resp -ExpectedHttpStatus 401 -ExpectedCode 401

$resp = Invoke-JsonApi -Method POST -Path "/api/v1/user/login" -Body @{
  username = $username
  password = ""
}
Assert-LoginResponse -Id "M1-UT-011" -Title "empty password" -Response $resp -ExpectedHttpStatus 401 -ExpectedCode 401

$resp = Invoke-JsonApi -Method POST -Path "/api/v1/user/login" -Body @{
  username = "missing_user_$runId"
  password = $password
}
Assert-LoginResponse -Id "M1-UT-012" -Title "nonexistent user" -Response $resp -ExpectedHttpStatus 401 -ExpectedCode 401

Write-Host ""
Write-Host "LoginTest summary: Total=$script:Total Passed=$script:Passed Failed=$script:Failed"

if ($script:Failed -gt 0) {
  Write-Host ""
  Write-Host "Failures:"
  $script:Failures | ForEach-Object { Write-Host $_ -ForegroundColor Red }
  exit 1
}

exit 0
