param(
  [int]$HttpPort = 8080,
  [int]$TcpPort = 9091,
  [int]$UdpPort = 9092
)

$ErrorActionPreference = "Continue"
$projectRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)

function Try-AddFirewallRule {
  param(
    [string]$Name,
    [string]$Protocol,
    [int]$Port
  )

  try {
    $check = netsh advfirewall firewall show rule name="$Name" | Out-String
    if ($check -match "No rules match") {
      netsh advfirewall firewall add rule name="$Name" dir=in action=allow protocol=$Protocol localport=$Port | Out-Null
      Write-Host "已添加防火墙规则: $Name"
    } else {
      Write-Host "防火墙规则已存在: $Name"
    }
  } catch {
    Write-Host "防火墙规则添加失败（可能需要管理员权限）: $Name"
  }
}

function Get-PublicIP {
  try {
    $ip = (Invoke-WebRequest -Uri "https://api.ipify.org" -UseBasicParsing -TimeoutSec 8).Content.Trim()
    if ($ip) { return $ip }
  } catch {}
  return ""
}

function Test-Http {
  param(
    [string]$Url,
    [int]$TimeoutSec = 4
  )
  try {
    $r = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec $TimeoutSec
    return ($r.StatusCode -ge 200 -and $r.StatusCode -lt 500)
  } catch {
    return $false
  }
}

Push-Location $projectRoot
try {
  Try-AddFirewallRule -Name "AI-Agent-HTTP-8080" -Protocol "TCP" -Port $HttpPort
  Try-AddFirewallRule -Name "AI-Agent-TCP-9091" -Protocol "TCP" -Port $TcpPort
  Try-AddFirewallRule -Name "AI-Agent-UDP-9092" -Protocol "UDP" -Port $UdpPort

  powershell -ExecutionPolicy Bypass -File ".\scripts\run.ps1" -HttpPort $HttpPort -TcpPort $TcpPort -UdpPort $UdpPort

  $publicIP = Get-PublicIP
  $lanIP = ""
  try {
    $cfg = Get-NetIPConfiguration | Where-Object { $_.IPv4Address -and $_.IPv4DefaultGateway } | Select-Object -First 1
    if ($cfg) {
      $lanIP = $cfg.IPv4Address.IPAddress
    }
  } catch {}

  if ($lanIP -ne "") {
    Write-Host "局域网访问地址: http://$lanIP`:$HttpPort/"
  }
  if ($publicIP -ne "") {
    Write-Host "公网访问地址: http://$publicIP`:$HttpPort/"
    $selfPublicOK = Test-Http -Url "http://$publicIP`:$HttpPort/" -TimeoutSec 5
    if (-not $selfPublicOK) {
      Write-Host "本机访问公网IP失败：通常是路由器不支持NAT回环(正常现象)。"
      Write-Host "请用局域网地址在本机测试；让外部网络设备测试公网地址。"
    }
    Write-Host "如外部网络访问失败，请在路由器做端口映射 $HttpPort -> 本机，并确认不是CGNAT。"
  } else {
    Write-Host "未获取到公网IP。可手动访问: https://ip.sb 查看后拼接端口 $HttpPort。"
  }
} finally {
  Pop-Location
}
