param(
  [string]$BaseUrl = "http://127.0.0.1:8080",
  [int]$TimeoutSec = 10,
  [int]$WrongPasswordAttempts = 20
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
$username = "login_edge_$runId"
$password = "123456"

Write-Host "Running LoginEdgeTest against $BaseUrl"

$setupResp = Invoke-JsonApi -Method POST -Path "/api/v1/user/register" -Body @{
  username = $username
  password = $password
}
if ($setupResp.StatusCode -ne 200 -or (Get-BodyCode $setupResp) -ne 0) {
  throw "LoginEdgeTest setup failed: seed user could not be registered. Raw: $($setupResp.Raw)"
}

$resp = Invoke-JsonApi -Method POST -Path "/api/v1/user/login" -Body @{
  username = "' OR 1=1 --"
  password = "anything"
}
$ok = ($resp.StatusCode -eq 401 -and (Get-BodyCode $resp) -eq 401)
Assert-Condition -Id "M1-EXT-012" -Title "SQL injection shaped username login" -Ok $ok -Failure "expected HTTP 401 and code=401, got HTTP $($resp.StatusCode), code=$(Get-BodyCode $resp)" -Response $resp

$resp = Invoke-JsonApi -Method POST -Path "/api/v1/user/login" -RawBody (@{ username = $username; password = 123456 } | ConvertTo-Json -Compress)
$code = Get-BodyCode $resp
$ok = ($resp.StatusCode -in @(400, 401) -and $code -in @(400, 401))
Assert-Condition -Id "M1-EXT-013" -Title "password type mismatch" -Ok $ok -Failure "expected controlled HTTP 400/401, got HTTP $($resp.StatusCode), code=$code" -Response $resp

$resp = Invoke-JsonApi -Method POST -Path "/api/v1/user/login" -Body @{
  username = $username
}
$code = Get-BodyCode $resp
$ok = ($resp.StatusCode -in @(400, 401) -and $code -in @(400, 401))
Assert-Condition -Id "M1-EXT-014" -Title "missing password field" -Ok $ok -Failure "expected controlled HTTP 400/401, got HTTP $($resp.StatusCode), code=$code" -Response $resp

$caseVariant = $username.ToUpperInvariant()
$resp = Invoke-JsonApi -Method POST -Path "/api/v1/user/login" -Body @{
  username = $caseVariant
  password = $password
}
$ok = ($resp.StatusCode -eq 401 -and (Get-BodyCode $resp) -eq 401)
Assert-Condition -Id "M1-EXT-015" -Title "different username casing" -Ok $ok -Failure "expected HTTP 401 and code=401, got HTTP $($resp.StatusCode), code=$(Get-BodyCode $resp)" -Response $resp

$badCount = 0
$unexpectedSuccess = 0
$acceptableAuthFailures = 0
$samples = @()
$sw = [System.Diagnostics.Stopwatch]::StartNew()
for ($i = 0; $i -lt $WrongPasswordAttempts; $i++) {
  $resp = Invoke-JsonApi -Method POST -Path "/api/v1/user/login" -Body @{
    username = $username
    password = "bad_password_$i"
  }
  $code = Get-BodyCode $resp
  if ($resp.StatusCode -eq 200 -or $code -eq 0) {
    $unexpectedSuccess++
    if ($samples.Count -lt 3) {
      $samples += "attempt=$i HTTP=$($resp.StatusCode) code=$code raw=$($resp.Raw)"
    }
  } elseif (($resp.StatusCode -eq 401 -and $code -eq 401) -or ($resp.StatusCode -eq 429 -and $code -eq 429)) {
    $acceptableAuthFailures++
  } else {
    $badCount++
    if ($samples.Count -lt 3) {
      $samples += "attempt=$i HTTP=$($resp.StatusCode) code=$code raw=$($resp.Raw) error=$($resp.Error)"
    }
  }
}
$sw.Stop()
$ok = ($unexpectedSuccess -eq 0 -and $badCount -eq 0 -and $acceptableAuthFailures -eq $WrongPasswordAttempts)
Assert-Condition -Id "M1-EXT-016" -Title "repeated wrong password attempts" -Ok $ok -Failure "expected all $WrongPasswordAttempts attempts to return 401 or 429, authFailures=$acceptableAuthFailures unexpectedSuccess=$unexpectedSuccess bad=$badCount elapsedMs=$($sw.ElapsedMilliseconds); samples=$($samples -join ' || ')"

Write-Host ""
Write-Host "LoginEdgeTest summary: Total=$script:Total Passed=$script:Passed Failed=$script:Failed"

if ($script:Failed -gt 0) {
  Write-Host ""
  Write-Host "Failures:"
  $script:Failures | ForEach-Object { Write-Host $_ -ForegroundColor Red }
  exit 1
}

exit 0
