param(
  [string]$BaseUrl = "http://127.0.0.1:8080",
  [int]$TimeoutSec = 10,
  [string]$OldToken = ""
)

$ErrorActionPreference = "Stop"

Add-Type -AssemblyName System.Net.Http

$script:Total = 0
$script:Passed = 0
$script:Failed = 0
$script:Skipped = 0
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

function Skip-Case {
  param(
    [string]$Id,
    [string]$Title,
    [string]$Reason
  )

  $script:Total++
  $script:Skipped++
  Write-Host "[SKIP] $Id $Title - $Reason" -ForegroundColor Yellow
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

Write-Host "Running AccountEdgeTest against $BaseUrl"

$resp = Invoke-JsonApi -Method GET -Path "/api/v1/user/me"
$ok = ($resp.StatusCode -eq 401 -and (Get-BodyCode $resp) -eq 401)
Assert-Condition -Id "M1-EXT-022" -Title "query account without token" -Ok $ok -Failure "expected HTTP 401 and code=401, got HTTP $($resp.StatusCode), code=$(Get-BodyCode $resp)" -Response $resp

$resp = Invoke-JsonApi -Method GET -Path "/api/v1/user/me" -Headers @{
  Authorization = "Bearer unknown"
}
$ok = ($resp.StatusCode -eq 401 -and (Get-BodyCode $resp) -eq 401)
Assert-Condition -Id "M1-EXT-023" -Title "query account with unknown token" -Ok $ok -Failure "expected HTTP 401 and code=401, got HTTP $($resp.StatusCode), code=$(Get-BodyCode $resp)" -Response $resp

if ([string]::IsNullOrWhiteSpace($OldToken)) {
  Skip-Case -Id "M1-EXT-024" -Title "old token after gateway restart" -Reason "pass -OldToken after restarting the gateway to verify this case"
} else {
  $resp = Invoke-JsonApi -Method GET -Path "/api/v1/user/me" -Headers @{
    Authorization = "Bearer $OldToken"
  }
  $ok = ($resp.StatusCode -eq 401 -and (Get-BodyCode $resp) -eq 401)
  Assert-Condition -Id "M1-EXT-024" -Title "old token after gateway restart" -Ok $ok -Failure "expected HTTP 401 and code=401, got HTTP $($resp.StatusCode), code=$(Get-BodyCode $resp)" -Response $resp
}

Write-Host ""
Write-Host "AccountEdgeTest summary: Total=$script:Total Passed=$script:Passed Failed=$script:Failed Skipped=$script:Skipped"

if ($script:Failed -gt 0) {
  Write-Host ""
  Write-Host "Failures:"
  $script:Failures | ForEach-Object { Write-Host $_ -ForegroundColor Red }
  exit 1
}

exit 0
